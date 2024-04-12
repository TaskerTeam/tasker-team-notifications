package main

import (
	"context"
	"database/sql"
	"errors"
	"example/grpc/pb"
	"log"
	"net"
	"strconv"

	"os"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Estrutura para representar uma notificação
type Notification struct {
	ID             int       `json:"id"`
	UserIDSend     int       `json:"id_user_send"`
	UserIDReceived int       `json:"id_user_received"`
	TypeMessage    int       `json:"type_message"`
	Message        string    `json:"message"`
	Date           time.Time `json:"date"`
}

type myNotificationServer struct {
	pb.UnimplementedNotificationServiceServer
}

// Função para estabelecer uma conexão com o banco de dados PostgreSQL
func connectDB() (*sql.DB, error) {
	connStr := "user=postgres dbname=grpc password=root sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Função para fechar a conexão com o banco de dados PostgreSQL
func closeDB(db *sql.DB) {
	db.Close()
}

// Método para obter todas as notificações
func (s *myNotificationServer) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationList, error) {
	// Configurar a conexão com o banco de dados PostgreSQL
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	// Consulta ao banco de dados para obter todas as notificações
	rows, err := db.Query("SELECT id, id_user_send, id_user_received, type_message, message, date FROM notifications")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Mapear as linhas do banco de dados para objetos de notificação protobuf
	notifications := []*pb.Notification{}
	for rows.Next() {
		var notification Notification
		err := rows.Scan(&notification.ID, &notification.UserIDSend, &notification.UserIDReceived, &notification.TypeMessage, &notification.Message, &notification.Date)
		if err != nil {
			return nil, err
		}

		// Criar um objeto de notificação protobuf e adicionar à lista
		pbNotification := &pb.Notification{
			Id:             int32(notification.ID),
			UserIdSend:     int32(notification.UserIDSend),
			UserIdReceived: int32(notification.UserIDReceived),
			TypeMessage:    int32(notification.TypeMessage),
			Message:        notification.Message,
			Date:           notification.Date.String(), // Convertendo para string para usar o formato esperado pelo protobuf
		}
		notifications = append(notifications, pbNotification)
	}

	// Verificar se ocorreu algum erro durante a leitura das linhas
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Retornar a lista de notificações como parte da resposta protobuf
	return &pb.NotificationList{
		Notifications: notifications,
	}, nil
}

// Método para obter uma notificação específica
func (s *myNotificationServer) GetNotification(ctx context.Context, req *pb.GetNotificationRequest) (*pb.Notification, error) {
	// Extrair o ID da notificação do pedido
	notificationID := req.GetId()

	// Configurar a conexão com o banco de dados PostgreSQL
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	// Consultar o banco de dados para obter a notificação com o ID fornecido
	var notification Notification
	err = db.QueryRow("SELECT id, id_user_send, id_user_received, type_message, message, date FROM notifications WHERE id = $1", notificationID).Scan(
		&notification.ID, &notification.UserIDSend, &notification.UserIDReceived, &notification.TypeMessage, &notification.Message, &notification.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "Notificação com ID %v não encontrada", notificationID)
		}
		return nil, err
	}

	// Construir e retornar a notificação encontrada
	pbNotification := &pb.Notification{
		Id:             int32(notification.ID),
		UserIdSend:     int32(notification.UserIDSend),
		UserIdReceived: int32(notification.UserIDReceived),
		TypeMessage:    int32(notification.TypeMessage),
		Message:        notification.Message,
		Date:           notification.Date.String(), // Convertendo para string para usar o formato esperado pelo protobuf
	}
	return pbNotification, nil
}

// Método para criar uma notificação
func (s *myNotificationServer) CreateNotification(ctx context.Context, req *pb.CreateNotificationRequest) (*pb.Notification, error) {
	if req == nil || req.Notification == nil {
		return nil, errors.New("empty request")
	}

	newNotification := req.GetNotification()

	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	var notificationID int
	err = db.QueryRow("INSERT INTO notifications (id_user_send, id_user_received, type_message, message, date) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP) RETURNING id",
		newNotification.UserIdSend, newNotification.UserIdReceived, newNotification.TypeMessage, newNotification.Message).Scan(&notificationID)
	if err != nil {
		return nil, err
	}

	createdNotification := &pb.Notification{
		Id:             int32(notificationID),
		UserIdSend:     newNotification.UserIdSend,
		UserIdReceived: newNotification.UserIdReceived,
		TypeMessage:    newNotification.TypeMessage,
		Message:        newNotification.Message,
		Date:           time.Now().String(),
	}
	return createdNotification, nil
}

// Método para atualizar uma notificação
func (s *myNotificationServer) UpdateNotification(ctx context.Context, req *pb.UpdateNotificationRequest) (*pb.Notification, error) {
	// Extrair a notificação do pedido
	updateNotification := req.GetNotification()
	notificationID := req.GetId()

	// Configurar a conexão com o banco de dados PostgreSQL
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	// Inicializar a lista de campos a serem atualizados
	var fieldsToUpdate []string

	// Inicializar a lista de valores a serem atualizados
	var valuesToUpdate []interface{}

	// Construir a consulta SQL base
	query := "UPDATE notifications SET "

	// Verificar quais campos foram fornecidos e adicionar à lista de campos e valores
	if updateNotification.UserIdSend != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "id_user_send")
		valuesToUpdate = append(valuesToUpdate, updateNotification.UserIdSend)
	}
	if updateNotification.UserIdReceived != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "id_user_received")
		valuesToUpdate = append(valuesToUpdate, updateNotification.UserIdReceived)
	}
	if updateNotification.TypeMessage != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "type_message")
		valuesToUpdate = append(valuesToUpdate, updateNotification.TypeMessage)
	}
	if updateNotification.Message != "" {
		fieldsToUpdate = append(fieldsToUpdate, "message")
		valuesToUpdate = append(valuesToUpdate, updateNotification.Message)
	}

	// Adicionar cada campo atualizável à consulta SQL
	for i, field := range fieldsToUpdate {
		query += field + " = $" + strconv.Itoa(i+1)
		if i < len(fieldsToUpdate)-1 {
			query += ", "
		}
	}

	// Adicionar a cláusula WHERE com o ID da notificação
	query += " WHERE id = $"
	query += strconv.Itoa(len(fieldsToUpdate) + 1)

	// Adicionar o ID da notificação à lista de valores
	valuesToUpdate = append(valuesToUpdate, notificationID)

	// Executar a consulta SQL dinâmica
	_, err = db.Exec(query, valuesToUpdate...)
	if err != nil {
		return nil, err
	}

	// Retornar a notificação atualizada
	updatedNotification := &pb.Notification{
		Id:             int32(notificationID),
		UserIdSend:     updateNotification.UserIdSend,
		UserIdReceived: updateNotification.UserIdReceived,
		TypeMessage:    updateNotification.TypeMessage,
		Message:        updateNotification.Message,
		Date:           time.Now().String(), // Definir a data atual
	}
	return updatedNotification, nil
}

// Método para excluir uma notificação
func (s *myNotificationServer) DeleteNotification(ctx context.Context, req *pb.DeleteNotificationRequest) (*pb.DeleteNotificationResponse, error) {
	// Obtenha o ID da notificação do pedido
	notificationID := req.GetId()

	// Configurar a conexão com o banco de dados PostgreSQL
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	// Executar a consulta SQL para excluir a notificação
	_, err = db.Exec("DELETE FROM notifications WHERE id = $1", notificationID)
	if err != nil {
		return nil, err
	}

	// Construir e retornar a resposta de exclusão bem-sucedida
	response := &pb.DeleteNotificationResponse{
		Success: true,
	}
	return response, nil
}

// Iniciar a aplicação
func main() {
	os.Setenv("CGO_ENABLED", "1")

	grpcServer := grpc.NewServer()

	pb.RegisterNotificationServiceServer(grpcServer, &myNotificationServer{})

	// Servidor de Notificações é executado na porta 5030
	lis, err := net.Listen("tcp", ":5053")
	if err != nil {
		log.Fatalf("Falha ao ouvir a porta 5053: %v", err)
	}
	defer lis.Close()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Falha ao iniciar o servidor gRPC: %v", err)
	}

	// Configurar a conexão com o banco de dados PostgreSQL
	connStr := "user=postgres dbname=grpc password=root sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic("Falha ao conectar ao banco de dados")
	}
	defer db.Close()

	// Crie a tabela se não existir
	createTable := `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		id_user_send INTEGER,
		id_user_received INTEGER,
		type_message INTEGER,
		message TEXT,
		date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		panic("Falha ao criar a tabela: " + err.Error())
	}

}
