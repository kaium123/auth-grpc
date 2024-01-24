package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	pb "auth/app/protos"

	_ "github.com/spf13/viper/remote"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func readConfig() {
	consulPath := os.Getenv("CONSUL_PATH")
	consulURL := os.Getenv("CONSUL_URL")
	// consulURL := "http://127.0.0.1:8500"
	// consulPath := "auth"

	viper.AddRemoteProvider("consul", consulURL, consulPath)
	viper.SetConfigType("json")

	err := viper.ReadRemoteConfig()
	if err != nil {
		panic(err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatal("Error unmarshalling config:", err)
	}

	viper.AddRemoteProvider("consul", "localhost:8500", "MY_CONSUL_KEY")

	fmt.Printf("Database Host: %s\n", config.Database.Host)
	fmt.Printf("Server Address: %s\n", config.Server.Address)

}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type ServerConfig struct {
	Address string `mapstructure:"address"`
	Port    string `mapstructure:"port"`
}

type AppConfig struct {
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
}

var Db *bun.DB

func ConnectDB() {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Database.Username, config.Database.Password,
		config.Database.Host, config.Database.Port, config.Database.DBName,
	)

	connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn))
	sqldb := sql.OpenDB(connector)
	db := bun.NewDB(sqldb, pgdialect.New())

	Db = db

	_, err := Db.NewCreateTable().Model(&Student{}).IfNotExists().Exec(context.Background())
	if err != nil {
		panic(err)
	}

	_, err = Db.NewCreateTable().Model(&ActivityLog{}).IfNotExists().Exec(context.Background())
	if err != nil {
		panic(err)
	}
}

var config AppConfig

func GetDB() *bun.DB {
	if Db == nil {
		ConnectDB()
	}
	return Db
}

type Server struct {
	pb.UnimplementedStudentServer
}

type Student struct {
	ID        int64     `json:"id" bun:"pk,autoincrement"`
	UserName  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at" bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `json:"updated_at" bun:",nullzero,notnull,default:current_timestamp"`
}

type ActivityLog struct {
	ID        int64     `json:"id" bun:"pk,autoincrement"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token" bun:"default:gen_random_uuid()"`
	CreatedAt time.Time `json:"created_at" bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `json:"updated_at" bun:",nullzero,notnull,default:current_timestamp"`
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequestBody) (*pb.LoginResponseBody, error) {
	var user Student
	db := GetDB()
	err := db.NewSelect().Model(&user).Where("user_name = ?", req.Username).Where("password = ?", req.Password).Scan(ctx)
	if err != nil {
		log.Printf("Error fetching user from database: %v", err)
		return nil, err
	}

	activityLog := &ActivityLog{}
	if user.UserName == req.Username && user.Password == req.Password {
		_, err := db.NewInsert().Model(activityLog).Exec(context.Background())
		if err != nil {
			log.Printf("Error inserting in  database: %v", err)
			return nil, err
		}
	}

	resp := &pb.LoginResponseBody{
		Id:    int32(user.ID),
		Msg:   "Login successful",
		Token: activityLog.Token,
	}

	return resp, nil
}

func (s *Server) TokenValidation(ctx context.Context, req *pb.Token) (*pb.GetTokenResponseBody, error) {

	activityLog := &ActivityLog{}
	db := GetDB()
	err := db.NewSelect().Model(activityLog).Where("token = ?", req.Token).Scan(ctx)
	if err != nil {
		log.Printf("Error fetching user from database: %v", err)
		return nil, err
	}

	currentTime := time.Now().UTC()
	expectedCreatedTime := currentTime.Add(time.Duration(-1*req.ExpirationTime) * time.Minute)

	if activityLog.CreatedAt.UTC().Before(expectedCreatedTime) {
		resp := &pb.GetTokenResponseBody{
			Message: "not authorized",
		}
		return resp, nil
	}

	resp := &pb.GetTokenResponseBody{
		Id:      int32(activityLog.ID),
		Message: "authenticated",
	}

	return resp, nil
}

func (s *Server) SignUp(ctx context.Context, req *pb.SignUpRequestBody) (*pb.SignUpResponseBody, error) {

	newUser := &Student{
		UserName: req.Username,
		Password: req.Password,
	}

	db := GetDB()
	_, err := db.NewInsert().Model(newUser).Exec(ctx)
	if err != nil {
		log.Printf("Error creating new user: %v", err)
		return nil, err
	}

	resp := &pb.SignUpResponseBody{
		Id:  int32(newUser.ID),
		Msg: "Signup successful",
	}

	return resp, nil
}

func main() {
	readConfig()
	// port := viper.GetString("APP_PORT")
	lis, err := net.Listen("tcp", ":"+config.Server.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	ConnectDB()

	s := grpc.NewServer()
	reflection.Register(s)

	pb.RegisterStudentServer(s, &Server{})

	fmt.Println("gRPC server is running on port " + config.Server.Port)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
