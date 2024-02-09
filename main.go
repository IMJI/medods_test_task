package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var port = ":3333"
var dbConnectionString = "mongodb://localhost:27017/"
var databaseName = "medods"
var collectionName = "user_refresh_tokens"

var secretKey = []byte("SECRET")
var jwtExpirationTime = 60 * time.Minute
var refreshExpirationTime = 30 * 24 * time.Hour

var collection *mongo.Collection

type ErrorMessage struct {
	StatusCode   int    `json:"status_code"`
	ErrorMessage string `json:"error_message"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	Guid         string `json:"guid"`
	RefreshToken string `json:"refresh_token"`
}

type UserRefreshToken struct {
	Guid           string    `bson:"_id"`
	RefreshToken   string    `bson:"refresh_token"`
	ExpirationTime time.Time `bson:"expiration_time"`
}

func handleError(w http.ResponseWriter, statusCode int, errorMessage string) {
	w.WriteHeader(statusCode)
	error := ErrorMessage{statusCode, errorMessage}
	json.NewEncoder(w).Encode(error)
}

func handleAuthRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "GET" {
		handleError(w, http.StatusMethodNotAllowed, "Метод запрещен")
		return
	}
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		handleError(w, http.StatusBadRequest, "Не указан GUID пользователя")
		return
	}
	jwtToken, err := generateJWT(guid)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	refreshToken := generateRefreshToken(guid)
	hashedRefreshToken, err := hashRefreshToken(refreshToken)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	upsertRefreshToken(*collection, guid, hashedRefreshToken)
	data := TokenPair{jwtToken, refreshToken}
	json.NewEncoder(w).Encode(data)
}

func handleRefreshRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		handleError(w, http.StatusMethodNotAllowed, "Метод запрещен")
		return
	}
	decoder := json.NewDecoder(r.Body)
	var bodyData TokenPair
	err := decoder.Decode(&bodyData)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	guid, err := decodeGuidFromJWT(bodyData.AccessToken)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	jwtToken, err := generateJWT(guid)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	filter := bson.D{{"_id", guid}}
	var result UserRefreshToken
	err = collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		handleError(w, http.StatusNotFound, "Пользователь которому принадлежит токен не найден")
		return
	}
	if time.Now().After(result.ExpirationTime) {
		handleError(w, http.StatusBadRequest, "Токен обновления устарел")
		return
	}
	if !checkRefreshTokenHash(bodyData.RefreshToken, result.RefreshToken) {
		handleError(w, http.StatusBadRequest, "Неверный токен обновления")
		return
	}
	refreshToken := generateRefreshToken(guid)
	hashedRefreshToken, err := hashRefreshToken(refreshToken)
	if err != nil {
		handleError(w, http.StatusInternalServerError, "Возникла ошибка сервера")
		return
	}
	upsertRefreshToken(*collection, guid, hashedRefreshToken)
	data := TokenPair{jwtToken, refreshToken}
	json.NewEncoder(w).Encode(data)
}

func generateJWT(guid string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS512)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(jwtExpirationTime)
	claims["guid"] = guid
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func decodeGuidFromJWT(jwtToken string) (string, error) {
	var claims jwt.MapClaims
	_, err := jwt.ParseWithClaims(jwtToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	v, _ := err.(*jwt.ValidationError)

	if v.Errors == jwt.ValidationErrorExpired {
		return claims["guid"].(string), nil
	}
	return "", err
}

func generateRefreshToken(guid string) string {
	t := time.Now().Format("15:04:05.00000")
	bytes := []byte(guid + t)
	refreshToken := base64.StdEncoding.EncodeToString(bytes)
	return refreshToken
}

func connectToDB() mongo.Client {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(dbConnectionString))
	if err != nil {
		log.Fatal(err)
	}
	return *client
}

func disconnectFromDB(client mongo.Client) {
	err := client.Disconnect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
}

func upsertRefreshToken(collection mongo.Collection, guid string, refreshToken string) {
	filter := bson.D{{"_id", guid}}
	update := bson.D{{"$set", bson.D{{"refresh_token", refreshToken}, {"expiration_time", time.Now().Add(refreshExpirationTime)}}}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		log.Fatal(err)
	}
}

func hashRefreshToken(refreshToken string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(refreshToken), 10)
	return string(bytes), err
}

func checkRefreshTokenHash(refreshToken string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(refreshToken))
	return err == nil
}

func main() {
	http.HandleFunc("/auth", handleAuthRoute)
	http.HandleFunc("/refresh", handleRefreshRoute)

	client := connectToDB()
	collection = client.Database(databaseName).Collection(collectionName)

	err := http.ListenAndServe(port, nil)
	if errors.Is(err, http.ErrServerClosed) {
		disconnectFromDB(client)
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}
