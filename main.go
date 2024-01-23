package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"

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
	// consulPath := os.Getenv("CONSUL_PATH")
	// consulURL := os.Getenv("CONSUL_URL")
	consulPath := "localhost:8500"
	consulURL := "auth"

	viper.AddRemoteProvider("consul", consulURL, consulPath)
	viper.SetConfigType("json") // Need to explicitly set this to json

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
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Database.Host, config.Database.Port, config.Database.Username,
		config.Database.Password, config.Database.DBName,
	)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	db := bun.NewDB(sqldb, pgdialect.New())

	// db.AutoMigrate(&Book{})
	Db = db
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
	ID       int64  `json:"id"`
	UserName string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

func (s *Server) Login(ctx context.Context, req *pb.LoginRequestBody) (*pb.LoginResponseBody, error) {
	var user Student
	db := GetDB()
	err := db.NewSelect().Model(&user).Where("username = ?", req.Username).Where("password = ?", req.Password).Scan(ctx)
	if err != nil {
		log.Printf("Error fetching user from database: %v", err)
		return nil, err
	}

	resp := &pb.LoginResponseBody{
		Id:    int32(user.ID),
		Msg:   "Login successful",
		Token: user.Token,
	}

	return resp, nil
}

func (s *Server) GetToken(ctx context.Context, req *pb.Token) (*pb.GetTokenResponseBody, error) {

	var user Student
	db := GetDB()
	err := db.NewSelect().Model(&user).Where("token = ?", req.Token).Scan(ctx)
	if err != nil {
		log.Printf("Error fetching user from database: %v", err)
		return nil, err
	}

	resp := &pb.GetTokenResponseBody{
		Id:       int32(user.ID),
		Username: user.UserName,
		Password: user.Password,
	}

	return resp, nil
}

func (s *Server) SignUp(ctx context.Context, req *pb.SignUpRequestBody) (*pb.LoginResponseBody, error) {

	newUser := Student{
		UserName: req.Username,
		Password: req.Password,
	}

	db := GetDB()
	_, err := db.NewInsert().Model(&newUser).Exec(ctx)
	if err != nil {
		log.Printf("Error creating new user: %v", err)
		return nil, err
	}

	resp := &pb.LoginResponseBody{
		Id:    int32(newUser.ID),
		Msg:   "Signup successful",
		Token: newUser.Token,
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
