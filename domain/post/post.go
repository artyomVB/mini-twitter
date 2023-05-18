package post

import "go.mongodb.org/mongo-driver/bson/primitive"

type Post struct {
	Id             string `json:"id" bson:"id"`
	Text           string `json:"text" bson:"text"`
	AuthorId       string `json:"authorId" bson:"authorId"`
	CreatedAt      string `json:"createdAt" bson:"createdAt"`
	LastModifiedAt string `json:"lastModifiedAt" bson:"lastModifiedAt"`
}

type PostWithOID struct {
	ID             primitive.ObjectID `bson:"_id"`
	Id             string             `bson:"id"`
	Text           string             `bson:"text"`
	AuthorId       string             `bson:"authorId"`
	CreatedAt      string             `bson:"createdAt"`
	LastModifiedAt string             `bson:"lastModifiedAt"`
}

func (pwo *PostWithOID) ToPost() Post {
	return Post{
		Id:             pwo.Id,
		AuthorId:       pwo.AuthorId,
		CreatedAt:      pwo.CreatedAt,
		Text:           pwo.Text,
		LastModifiedAt: pwo.LastModifiedAt,
	}
}
