Pra rodar o projeto você precisa instalar o Go:
```https://go.dev/doc/install```

Após a instalação do Go, usando o terminal na raiz do diretório:

Instalar todas as depedências
```go mod tidy```

Em cmd/server/api.go, localize o trecho de código:

```// Função para estabelecer uma conexão com o banco de dados PostgreSQL
func connectDB() (*sql.DB, error) {
	connStr := "user=postgres dbname=grpc password=root sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}
```

E alterar os parâmetros de acordo com seu PostgreSQL 
```user=postgres - usuário postgre
dbname=grpc - nome do banco de dados
password=root - senha do postgre
```
Em seguida rodar o projeto
```go run cmd/server/api.go```

Porta utilizada para rodar o projeto
```localhost:5053```
