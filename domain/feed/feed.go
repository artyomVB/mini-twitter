package feed

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"mini-twitter/domain/post"
)

type Feed struct {
	UserId         string             `bson:"userId"`
	Id             string             `bson:"id"`
	Text           string             `bson:"text"`
	AuthorId       string             `bson:"authorId"`
	CreatedAt      string             `bson:"createdAt"`
	LastModifiedAt string             `bson:"lastModifiedAt"`
	Oid            primitive.ObjectID `bson:"oid"`
}

func (f *Feed) ToPost() post.Post {
	return post.Post{
		Id:             f.Id,
		Text:           f.Text,
		AuthorId:       f.AuthorId,
		CreatedAt:      f.CreatedAt,
		LastModifiedAt: f.LastModifiedAt,
	}
}
