package main

import (
	"context"
	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"mini-twitter/api"
	"mini-twitter/domain/post"
	"mini-twitter/domain/subscribers"
	"os"
	"sync"
)

var mu1 sync.Mutex
var mu2 sync.Mutex

func processSubscribe(subscribee, subscriber string) error {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		return err
	}
	fd := client.Database(os.Getenv("MONGO_DBNAME")).Collection("feed")
	posts := client.Database(os.Getenv("MONGO_DBNAME")).Collection("posts")

	filter := bson.D{{"authorId", subscribee}}
	cur, _ := posts.Find(ctx, filter)
	ok := true

	for true {
		mu2.Lock()
		ok = cur.Next(ctx)
		if !ok {
			mu2.Unlock()
			break
		}
		var p post.PostWithOID
		_ = cur.Decode(&p)

		if err != nil {
			mu2.Unlock()
			return err
		}

		flag := true
		opts := options.UpdateOptions{Upsert: &flag}
		_, _ = fd.UpdateOne(ctx,
			bson.D{
				{"userId", subscriber},
				{"oid", p.ID},
			},
			bson.D{
				{"$set",
					bson.D{
						{"id", p.Id},
						{"text", p.Text},
						{"lastModifiedAt", p.LastModifiedAt},
						{"authorId", p.AuthorId},
						{"createdAt", p.CreatedAt},
					},
				},
			},
			&opts)
		mu2.Unlock()
	}
	return nil
}

func processModifyPost(Id, AuthorId, Text, CreatedAt, LastModifiedAt, Oid string) error {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		return err
	}
	fd := client.Database(os.Getenv("MONGO_DBNAME")).Collection("feed")
	subs := client.Database(os.Getenv("MONGO_DBNAME")).Collection("subscribers")

	var s subscribers.Subscribers
	err = subs.FindOne(ctx, bson.D{{"user", AuthorId}}).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	if err != nil {
		return err
	}

	_oid, _ := primitive.ObjectIDFromHex(Oid)
	filter := bson.D{{"oid", _oid}}
	update := bson.D{{"$set", bson.D{{"text", Text}, {"lastModifiedAt", LastModifiedAt}}}}
	mu1.Lock()
	mu2.Lock()
	_, _ = fd.UpdateMany(ctx, filter, update)
	mu2.Unlock()
	mu1.Unlock()
	return nil
}

func processNewPost(Id, AuthorId, Text, CreatedAt, LastModifiedAt, Oid string) error {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		return err
	}
	fd := client.Database(os.Getenv("MONGO_DBNAME")).Collection("feed")
	subs := client.Database(os.Getenv("MONGO_DBNAME")).Collection("subscribers")
	posts := client.Database(os.Getenv("MONGO_DBNAME")).Collection("posts")

	var s subscribers.Subscribers
	err = subs.FindOne(ctx, bson.D{{"user", AuthorId}}).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	if err != nil {
		return err
	}

	_oid, _ := primitive.ObjectIDFromHex(Oid)
	for _, elem := range s.Subscribers {
		var p post.PostWithOID
		mu1.Lock()
		_ = posts.FindOne(ctx, bson.D{{"id", Id}}).Decode(&p)
		flag := true
		opts := options.UpdateOptions{Upsert: &flag}
		_, _ = fd.UpdateOne(ctx,
			bson.D{
				{"userId", elem},
				{"oid", _oid},
			},
			bson.D{
				{"$set",
					bson.D{
						{"id", p.Id},
						{"text", p.Text},
						{"lastModifiedAt", p.LastModifiedAt},
						{"authorId", p.AuthorId},
						{"createdAt", p.CreatedAt},
					},
				},
			},
			&opts)
		mu1.Unlock()
	}
	return nil
}

func startServer() (*machinery.Server, error) {
	var cnf = &config.Config{
		Broker:          "redis://" + os.Getenv("REDIS_URL"),
		DefaultQueue:    "machinery_tasks",
		ResultBackend:   "redis://" + os.Getenv("REDIS_URL"),
		ResultsExpireIn: 3600,
		Redis: &config.RedisConfig{
			MaxIdle:                3,
			IdleTimeout:            240,
			ReadTimeout:            15,
			WriteTimeout:           15,
			ConnectTimeout:         15,
			NormalTasksPollPeriod:  1000,
			DelayedTasksPollPeriod: 500,
		},
	}

	server, err := machinery.NewServer(cnf)
	if err != nil {
		panic("Fatal error")
	}
	tasks := map[string]interface{}{
		"create":    processNewPost,
		"modify":    processModifyPost,
		"subscribe": processSubscribe,
	}

	_ = server.RegisterTasks(tasks)

	return server, nil
}

func main() {
	if os.Getenv("APP_MODE") == "SERVER" {
		serevr, _ := startServer()
		srv := api.MakeServer(serevr)
		log.Fatal(srv.ListenAndServe())
	} else {
		server, _ := startServer()
		worker := server.NewWorker("machinery_worker", 10)
		_ = worker.Launch()
	}
}
