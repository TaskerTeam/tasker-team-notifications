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
	ID          int       `json:"id"`
	TypeMessage int       `json:"type_message"`
	Message     string    `json:"message"`
	TaskTitle   string    `json:"task_title"` // Adicionando o campo TaskTitle
	Date        time.Time `json:"date"`
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

// GET - Todas as notificações
func (s *myNotificationServer) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.NotificationList, error) {
	// Configurar a conexão com o banco de dados PostgreSQL
	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	// Consulta ao banco de dados para obter todas as notificações
	rows, err := db.Query("SELECT id, type_message, message, task_title, date FROM notifications")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Mapear as linhas do banco de dados para objetos de notificação protobuf
	notifications := []*pb.Notification{}
	for rows.Next() {
		var notification Notification
		err := rows.Scan(&notification.ID, &notification.TypeMessage, &notification.Message, &notification.TaskTitle, &notification.Date)
		if err != nil {
			return nil, err
		}

		// Criar um objeto de notificação protobuf e adicionar à lista
		pbNotification := &pb.Notification{
			Id:          int32(notification.ID),
			TypeMessage: int32(notification.TypeMessage),
			Message:     notification.Message,
			TaskTitle:   notification.TaskTitle,
			Date:        notification.Date.String(),
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

// GET - Uma notificação
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
	err = db.QueryRow("SELECT id, type_message, message, task_title, date FROM notifications WHERE id = $1", notificationID).Scan(
		&notification.ID, &notification.TypeMessage, &notification.Message, &notification.TaskTitle, &notification.Date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "Notificação com ID %v não encontrada", notificationID)
		}
		return nil, err
	}

	// Construir e retornar a notificação encontrada
	pbNotification := &pb.Notification{
		Id:          int32(notification.ID),
		TypeMessage: int32(notification.TypeMessage),
		Message:     notification.Message,
		TaskTitle:   notification.TaskTitle,
		Date:        notification.Date.String(),
	}
	return pbNotification, nil
}

// POST - Criar uma notificação
func (s *myNotificationServer) CreateNotification(ctx context.Context, req *pb.CreateNotificationRequest) (*pb.Notification, error) {
	if req == nil || req.Notification == nil {
		return nil, errors.New("empty request")
	}

	newNotification := req.GetNotification()

	// Verificar se os campos obrigatórios estão presentes e não vazios
	if newNotification.TypeMessage == 0 || newNotification.Message == "" {
		return nil, errors.New("missing required fields")
	}

	db, err := connectDB()
	if err != nil {
		return nil, err
	}
	defer closeDB(db)

	var notificationID int
	err = db.QueryRow("INSERT INTO notifications (type_message, message, task_title, date) VALUES ($1, $2, $3, CURRENT_TIMESTAMP) RETURNING id",
		newNotification.TypeMessage, newNotification.Message, newNotification.TaskTitle).Scan(&notificationID)
	if err != nil {
		return nil, err
	}

	createdNotification := &pb.Notification{
		Id:          int32(notificationID),
		TypeMessage: newNotification.TypeMessage,
		Message:     newNotification.Message,
		TaskTitle:   newNotification.TaskTitle,
		Date:        time.Now().String(),
	}
	return createdNotification, nil
}

// PATCH - Atualizar uma notificação
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
	if updateNotification.TypeMessage != 0 {
		fieldsToUpdate = append(fieldsToUpdate, "type_message")
		valuesToUpdate = append(valuesToUpdate, updateNotification.TypeMessage)
	}
	if updateNotification.Message != "" {
		fieldsToUpdate = append(fieldsToUpdate, "message")
		valuesToUpdate = append(valuesToUpdate, updateNotification.Message)
	}
	if updateNotification.TaskTitle != "" { // Verificar se o título da tarefa foi fornecido
		fieldsToUpdate = append(fieldsToUpdate, "task_title")
		valuesToUpdate = append(valuesToUpdate, updateNotification.TaskTitle)
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
		Id:          int32(notificationID),
		TypeMessage: updateNotification.TypeMessage,
		Message:     updateNotification.Message,
		TaskTitle:   updateNotification.TaskTitle,
		Date:        time.Now().String(),
	}
	return updatedNotification, nil
}

// DELETE - Deletar uma notificação
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
		type_message INTEGER,
		message TEXT,
		task_title TEXT, 
		date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		panic("Falha ao criar a tabela: " + err.Error())
	}

}
