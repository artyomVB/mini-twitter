package storage

import (
	"context"
	"mini-twitter/domain/post"
)

type Storage interface {
	GetPostById(ctx context.Context, postId string) (*post.Post, error)
	AddPost(ctx context.Context, userId string, p *post.Post)
	GetPostsByUserId(ctx context.Context, userId string, token string, size int) ([]*post.Post, string, error)
	ModifyPost(ctx context.Context, userId string, postId string, newPost *post.Post) (*post.Post, error)
	Subscribe(ctx context.Context, subscribee string, subscriber string) error
	GetSubscribers(ctx context.Context, userId string) ([]string, error)
	GetSubscriptions(ctx context.Context, userId string) ([]string, error)
	GetFeed(ctx context.Context, userId string, token string, size int) ([]*post.Post, string, error)
}
