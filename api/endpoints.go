package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RichardKnop/machinery/v1"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"mini-twitter/domain/post"
	"mini-twitter/storage"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PostsByUserId map[string]any

func MakeServer(s *machinery.Server) *http.Server {
	r := mux.NewRouter()

	handler := NewHTTPHandler(s)

	r.HandleFunc("/api/v1/posts/{postId}", handler.GetPostById).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/posts", handler.CreatePost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/{userId}/posts", handler.GetPostsByUserId).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/posts/{postId}", handler.ModifyPost).Methods(http.MethodPatch)
	r.HandleFunc("/api/v1/users/{userId}/subscribe", handler.Subscribe).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/subscriptions", handler.GetSubscriptions).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/subscribers", handler.GetSubscribers).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/feed", handler.GetFeed).Methods(http.MethodGet)

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("0.0.0.0:%s", os.Getenv("SERVER_PORT")),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	return srv
}

func NewHTTPHandler(s *machinery.Server) *HTTPHandler {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		panic(err)
	}
	posts := client.Database(os.Getenv("MONGO_DBNAME")).Collection("posts")
	feed := client.Database(os.Getenv("MONGO_DBNAME")).Collection("feed")
	subscriptions := client.Database(os.Getenv("MONGO_DBNAME")).Collection("subscriptions")
	subscribers := client.Database(os.Getenv("MONGO_DBNAME")).Collection("subscribers")
	return &HTTPHandler{storage: &storage.MongoStorage{
		Posts:         posts,
		Feed:          feed,
		Subscriptions: subscriptions,
		Subscribers:   subscribers,
		Server:        s,
	}}
}

type HTTPHandler struct {
	storageType string
	storage     storage.Storage
}

func (h *HTTPHandler) CreatePost(rw http.ResponseWriter, r *http.Request) {
	var newPost post.Post
	_ = json.NewDecoder(r.Body).Decode(&newPost)
	userId := r.Header.Get("User-Id")
	if !validateUserId(userId) {
		response := ErrorResponse{"Invalid or empty user id"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusUnauthorized)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	h.storage.AddPost(r.Context(), userId, &newPost)
	rw.Header().Set("Content-Type", "application/json")
	ans, _ := json.Marshal(newPost)
	_, _ = rw.Write(ans)
}

func (h *HTTPHandler) GetPostById(rw http.ResponseWriter, r *http.Request) {
	postId := strings.Split(r.URL.Path, "/")[4]
	p, err := h.storage.GetPostById(r.Context(), postId)
	if err != nil {
		response := ErrorResponse{"Post not found"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNotFound)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	ans, _ := json.Marshal(*p)
	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(ans)
}

func (h *HTTPHandler) GetPostsByUserId(rw http.ResponseWriter, r *http.Request) {
	userId := strings.Split(r.URL.Path, "/")[4]
	pageToken := r.URL.Query().Get("page")
	sizeStr := r.URL.Query().Get("size")
	var size = storage.DEFAULT
	var err error
	if sizeStr != "" {
		size, err = strconv.Atoi(sizeStr)
		if err != nil || size <= 0 || size > 100 {
			response := ErrorResponse{"Invalid size"}
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusBadRequest)
			rawResponse, _ := json.Marshal(response)
			_, _ = rw.Write(rawResponse)
			return
		}
	}
	arr, nextToken, err := h.storage.GetPostsByUserId(r.Context(), userId, pageToken, size)
	if err != nil {
		response := ErrorResponse{"Invalid token"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	ans := make(PostsByUserId)
	if nextToken != "" {
		ans["nextPage"] = nextToken
	}
	ans["posts"] = arr
	ansStr, _ := json.Marshal(ans)
	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(ansStr)
}

func (h *HTTPHandler) ModifyPost(rw http.ResponseWriter, r *http.Request) {
	var newPost post.Post
	_ = json.NewDecoder(r.Body).Decode(&newPost)
	userId := r.Header.Get("User-Id")
	if !validateUserId(userId) {
		response := ErrorResponse{"Invalid or empty user id"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusUnauthorized)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	postId := strings.Split(r.URL.Path, "/")[4]
	modifiedPost, err := h.storage.ModifyPost(r.Context(), userId, postId, &newPost)
	if err == storage.ErrForbiddenAccess {
		response := ErrorResponse{"Forbidden access"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusForbidden)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	if err == storage.ErrPostNotFound {
		response := ErrorResponse{"Not found"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusNotFound)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	ans, _ := json.Marshal(modifiedPost)
	_, _ = rw.Write(ans)
}

func (h *HTTPHandler) Subscribe(rw http.ResponseWriter, r *http.Request) {
	subscriber := r.Header.Get("User-Id")
	subscribee := strings.Split(r.URL.Path, "/")[4]
	err := h.storage.Subscribe(r.Context(), subscribee, subscriber)
	if err != nil {
		response := ErrorResponse{"Bad request"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
}

func (h *HTTPHandler) GetSubscribers(rw http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("User-Id")
	if user == "" {
		response := ErrorResponse{"Bad request"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	subscribers, _ := h.storage.GetSubscribers(r.Context(), user)
	rw.Header().Set("Content-Type", "application/json")
	a := make(map[string]any)
	a["users"] = subscribers
	ans, _ := json.Marshal(a)
	_, _ = rw.Write(ans)
}

func (h *HTTPHandler) GetSubscriptions(rw http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("User-Id")
	if user == "" {
		response := ErrorResponse{"Bad request"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	subscriptions, _ := h.storage.GetSubscriptions(r.Context(), user)
	rw.Header().Set("Content-Type", "application/json")
	a := make(map[string]any)
	a["users"] = subscriptions
	ans, _ := json.Marshal(a)
	_, _ = rw.Write(ans)
}

func (h *HTTPHandler) GetFeed(rw http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("User-Id")
	if userId == "" {
		response := ErrorResponse{"Invalid user id"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	pageToken := r.URL.Query().Get("page")
	sizeStr := r.URL.Query().Get("size")
	var size = storage.DEFAULT
	var err error
	if sizeStr != "" {
		size, err = strconv.Atoi(sizeStr)
		if err != nil || size <= 0 || size > 100 {
			response := ErrorResponse{"Invalid size"}
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusBadRequest)
			rawResponse, _ := json.Marshal(response)
			_, _ = rw.Write(rawResponse)
			return
		}
	}
	arr, nextToken, err := h.storage.GetFeed(r.Context(), userId, pageToken, size)
	if err != nil {
		response := ErrorResponse{"Invalid token"}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		rawResponse, _ := json.Marshal(response)
		_, _ = rw.Write(rawResponse)
		return
	}
	ans := make(PostsByUserId)
	if nextToken != "" {
		ans["nextPage"] = nextToken
	}
	ans["posts"] = arr
	ansStr, _ := json.Marshal(ans)
	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(ansStr)
}

func validateUserId(userId string) bool {
	r := regexp.MustCompile("^[0-9a-f]+$")
	return r.MatchString(userId)
}
