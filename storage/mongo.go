package storage

import (
	"context"
	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/tasks"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"mini-twitter/domain/feed"
	"mini-twitter/domain/post"
	"mini-twitter/domain/subscribers"
	"mini-twitter/domain/subscriptions"
	"mini-twitter/utils"
	"strconv"
	"strings"
)

type MongoStorage struct {
	Posts         *mongo.Collection
	Feed          *mongo.Collection
	Subscriptions *mongo.Collection
	Subscribers   *mongo.Collection
	Server        *machinery.Server
}

func (m *MongoStorage) GetPostById(ctx context.Context, postId string) (*post.Post, error) {
	var p post.Post
	err := m.Posts.FindOne(ctx, bson.M{"id": postId}).Decode(&p)
	return &p, err
}

func (m *MongoStorage) AddPost(ctx context.Context, userId string, p *post.Post) {
	p.CreatedAt = utils.GetCurrentTimestamp()
	p.LastModifiedAt = utils.GetCurrentTimestamp()
	p.AuthorId = userId
	for true {
		p.Id = utils.GeneratePostId()
		sr := m.Posts.FindOne(ctx, bson.M{"id": p.Id})
		if sr.Err() != nil {
			break
		}
	}
	insertRes, _ := m.Posts.InsertOne(ctx, *p)

	signature := &tasks.Signature{
		Name: "create",
		Args: []tasks.Arg{
			{
				Type:  "string",
				Value: p.Id,
			},
			{
				Type:  "string",
				Value: p.AuthorId,
			},
			{
				Type:  "string",
				Value: p.Text,
			},
			{
				Type:  "string",
				Value: p.CreatedAt,
			},
			{
				Type:  "string",
				Value: p.LastModifiedAt,
			},
			{
				Type:  "string",
				Value: insertRes.InsertedID.(primitive.ObjectID).Hex(),
			},
		},
	}
	_, _ = m.Server.SendTask(signature)
}

func (m *MongoStorage) GetPostsByUserId(ctx context.Context, userId string, token string, size int) ([]*post.Post, string, error) {
	arr := make([]*post.Post, 0)
	var filter = bson.M{"authorId": userId}
	if token != "" {
		SizeAndPostId := strings.SplitN(token, "-", 2)
		if len(SizeAndPostId) != 2 {
			return arr, "", ErrParseToken
		}
		if size == DEFAULT {
			size, _ = strconv.Atoi(SizeAndPostId[0])
		}
		postId := SizeAndPostId[1]
		oid, _ := primitive.ObjectIDFromHex(postId)
		var p post.Post
		err := m.Posts.FindOne(ctx, bson.M{"_id": oid}).Decode(&p)
		if err != nil || p.AuthorId != userId {
			return arr, "", ErrParseToken
		}
		filter = bson.M{"$and": bson.A{bson.M{"authorId": userId}, bson.D{{"_id", bson.M{"$lt": oid}}}}}
	}
	opt := options.Find()
	opt.SetSort(bson.D{{"_id", -1}})
	cur, _ := m.Posts.Find(ctx, filter, opt)
	var ok = true
	if size == DEFAULT {
		size = 10
	}
	tokenStart := strconv.Itoa(size) + "-"
	retToken := ""
	for size > 0 {
		ok = cur.Next(ctx)
		if !ok {
			retToken = ""
			break
		}
		var pwo post.PostWithOID
		_ = cur.Decode(&pwo)
		retToken = tokenStart + pwo.ID.Hex()
		p := pwo.ToPost()
		arr = append(arr, &p)
		size--
	}
	if retToken != "" {
		if !cur.Next(ctx) {
			retToken = ""
		}
	}
	return arr, retToken, nil
}

func (m *MongoStorage) ModifyPost(ctx context.Context, userId string, postId string, newPost *post.Post) (*post.Post, error) {
	filter := bson.D{{"id", postId}, {"authorId", userId}}
	update := bson.D{{"$set", bson.D{{"text", newPost.Text}, {"lastModifiedAt", utils.GetCurrentTimestamp()}}}}
	var updatedPost post.PostWithOID
	opt := options.FindOneAndUpdate()
	after := options.After
	opt.ReturnDocument = &after
	err := m.Posts.FindOneAndUpdate(ctx, filter, update, opt).Decode(&updatedPost)
	if err == nil {
		signature := &tasks.Signature{
			Name: "modify",
			Args: []tasks.Arg{
				{
					Type:  "string",
					Value: updatedPost.Id,
				},
				{
					Type:  "string",
					Value: updatedPost.AuthorId,
				},
				{
					Type:  "string",
					Value: updatedPost.Text,
				},
				{
					Type:  "string",
					Value: updatedPost.CreatedAt,
				},
				{
					Type:  "string",
					Value: updatedPost.LastModifiedAt,
				},
				{
					Type:  "string",
					Value: updatedPost.ID.Hex(),
				},
			},
		}
		_, _ = m.Server.SendTask(signature)
		updatedPostWithoutOID := updatedPost.ToPost()
		return &updatedPostWithoutOID, nil
	}
	filterWithoutAuthorId := bson.D{{"id", postId}}
	err = m.Posts.FindOne(ctx, filterWithoutAuthorId).Err()
	if err != nil {
		return nil, ErrPostNotFound
	}
	return nil, ErrForbiddenAccess
}

func (m *MongoStorage) Subscribe(ctx context.Context, subscribee string, subscriber string) error {
	if subscribee == subscriber {
		return ErrInvalidSubscribe
	}
	flag := true
	err := m.Subscribers.FindOne(ctx, bson.D{
		{"user", subscribee},
	}).Err()
	if err != nil {
		newS := subscribers.Subscribers{UserId: subscribee, Subscribers: make([]string, 0)}
		newS.Subscribers = append(newS.Subscribers, subscriber)
		_, _ = m.Subscribers.InsertOne(ctx, newS)
	} else {
		filter := bson.D{{"user", subscribee}, {"subscribers", bson.M{"$not": bson.M{"$eq": subscriber}}}}
		update := bson.D{{"$push", bson.M{"subscribers": subscriber}}}
		updateRes, err2 := m.Subscribers.UpdateOne(ctx, filter, update)
		if err2 != nil {
			return err
		}
		if updateRes.MatchedCount == 0 {
			flag = false
		}
	}

	err = m.Subscriptions.FindOne(ctx, bson.D{
		{"user", subscriber},
	}).Err()
	if err != nil {
		newS := subscriptions.Subscriptions{UserId: subscriber, Subscriptions: make([]string, 0)}
		newS.Subscriptions = append(newS.Subscriptions, subscribee)
		_, _ = m.Subscriptions.InsertOne(ctx, newS)
	} else {
		filter := bson.D{{"user", subscriber}, {"subscriptions", bson.M{"$not": bson.M{"$eq": subscribee}}}}
		update := bson.D{{"$push", bson.M{"subscriptions": subscribee}}}
		_, err = m.Subscriptions.UpdateOne(ctx, filter, update)
		if err != nil {
			return err
		}
	}
	signature := &tasks.Signature{
		Name: "subscribe",
		Args: []tasks.Arg{
			{
				Type:  "string",
				Value: subscribee,
			},
			{
				Type:  "string",
				Value: subscriber,
			},
		},
	}
	if flag {
		_, _ = m.Server.SendTask(signature)
	}
	return nil
}

func (m *MongoStorage) GetSubscribers(ctx context.Context, userId string) ([]string, error) {
	var s subscribers.Subscribers
	filter := bson.D{{"user", userId}}
	err := m.Subscribers.FindOne(ctx, filter).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return []string{}, nil
	}
	return s.Subscribers, err
}

func (m *MongoStorage) GetSubscriptions(ctx context.Context, userId string) ([]string, error) {
	var s subscriptions.Subscriptions
	filter := bson.D{{"user", userId}}
	err := m.Subscriptions.FindOne(ctx, filter).Decode(&s)
	if err == mongo.ErrNoDocuments {
		return []string{}, nil
	}
	return s.Subscriptions, err
}

func (m *MongoStorage) GetFeed(ctx context.Context, userId string, token string, size int) ([]*post.Post, string, error) {
	arr := make([]*post.Post, 0)
	var filter = bson.M{"userId": userId}
	if token != "" {
		SizeAndFeedId := strings.SplitN(token, "-", 2)
		if len(SizeAndFeedId) != 2 {
			return arr, "", ErrParseToken
		}
		if size == DEFAULT {
			size, _ = strconv.Atoi(SizeAndFeedId[0])
		}
		feedId := SizeAndFeedId[1]
		oid, _ := primitive.ObjectIDFromHex(feedId)
		var f feed.Feed
		err := m.Feed.FindOne(ctx, bson.M{"oid": oid}).Decode(&f)
		if err != nil {
			return arr, "", ErrParseToken
		}
		filter = bson.M{"$and": bson.A{bson.M{"userId": userId}, bson.D{{"oid", bson.M{"$lt": oid}}}}}
	}
	opt := options.Find()
	opt.SetSort(bson.D{{"oid", -1}})
	cur, _ := m.Feed.Find(ctx, filter, opt)
	var ok = true
	if size == DEFAULT {
		size = 10
	}
	tokenStart := strconv.Itoa(size) + "-"
	retToken := ""
	for size > 0 {
		ok = cur.Next(ctx)
		if !ok {
			retToken = ""
			break
		}
		var f feed.Feed
		_ = cur.Decode(&f)
		retToken = tokenStart + f.Oid.Hex()
		p := f.ToPost()
		arr = append(arr, &p)
		size--
	}
	if retToken != "" {
		if !cur.Next(ctx) {
			retToken = ""
		}
	}
	return arr, retToken, nil
}
