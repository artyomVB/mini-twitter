package storage

import (
	_ "embed"
	"encoding/json"
	"mini-twitter/domain/post"
	"strconv"
	"time"
)
import "context"
import "github.com/go-redis/redis/v8"

//go:embed script.lua
var scr string

type CachedStorage struct {
	Client          *redis.Client
	InternalStorage Storage
}

func (cs *CachedStorage) GetPostById(ctx context.Context, postId string) (*post.Post, error) {
	var p *post.Post
	p = cs.getByPIDKey(ctx, cs.postIdKey(postId))
	if p != nil {
		return p, nil
	}
	var err error
	p, err = cs.InternalStorage.GetPostById(ctx, postId)
	if err != nil {
		return nil, err
	}
	cs.storeByPID(ctx, p)
	return p, err
}

func (cs *CachedStorage) storeByPID(ctx context.Context, p *post.Post) {
	res, _ := json.Marshal(p)
	_ = cs.Client.Set(ctx, cs.postIdKey(p.Id), string(res), time.Hour)
}

func (cs *CachedStorage) storeByUIDTokenSize(ctx context.Context, posts []*post.Post, newToken string, userId string, token string, size int) {
	var res []byte
	tmp := make(map[string]any)
	if newToken != "" {
		tmp["token"] = newToken
	}
	tmp["posts"] = posts
	res, _ = json.Marshal(tmp)
	cs.Client.Set(ctx, cs.uidTokenSizeKey(userId, token, size), string(res), time.Hour)
}

func (cs *CachedStorage) getByPIDKey(ctx context.Context, key string) *post.Post {
	res := cs.Client.Get(ctx, key)
	r, err := res.Result()
	if err == nil {
		var p post.Post
		_ = json.Unmarshal([]byte(r), &p)
		return &p
	}
	return nil
}

func (cs *CachedStorage) getByUIDTokenSizeKey(ctx context.Context, key string) ([]*post.Post, string, error) {
	res := cs.Client.Get(ctx, key)
	r, err := res.Result()
	if err == nil {
		var parsed map[string]any
		_ = json.Unmarshal([]byte(r), &parsed)
		var psts []post.Post
		somebytes, _ := json.Marshal(parsed["posts"])
		_ = json.Unmarshal(somebytes, &psts)
		token, ok := parsed["token"].(string)
		if !ok {
			token = ""
		}
		var posts []*post.Post
		for _, p := range psts {
			tmpPost := p
			posts = append(posts, &tmpPost)
		}
		return posts, token, nil
	}
	return nil, "", ErrCacheMiss
}

func (cs *CachedStorage) findAndDeleteByPID(ctx context.Context, key string) {
	cs.Client.Del(ctx, key)
}

func (cs *CachedStorage) findAndDeleteByUID(ctx context.Context, userId string) {
	cs.Client.Eval(ctx, scr, []string{"uts:" + userId + "*"})
}

func (cs *CachedStorage) postIdKey(postId string) string {
	return "pid:" + postId
}

func (cs *CachedStorage) uidTokenSizeKey(userId string, token string, size int) string {
	return "uts:" + userId + ":" + token + ":" + strconv.Itoa(size)
}

func (cs *CachedStorage) AddPost(ctx context.Context, userId string, p *post.Post) {
	cs.InternalStorage.AddPost(ctx, userId, p)
	cs.findAndDeleteByUID(ctx, userId)
	cs.storeByPID(ctx, p)
}

func (cs *CachedStorage) GetPostsByUserId(ctx context.Context, userId string, token string, size int) ([]*post.Post, string, error) {
	posts, newToken, err := cs.getByUIDTokenSizeKey(ctx, cs.uidTokenSizeKey(userId, token, size))
	if err == nil {
		return posts, newToken, nil
	}
	posts, newToken, err = cs.InternalStorage.GetPostsByUserId(ctx, userId, token, size)
	if err != nil {
		return posts, newToken, err
	}
	if size == DEFAULT {
		size = 10
	}
	cs.storeByUIDTokenSize(ctx, posts, newToken, userId, token, size)
	return posts, newToken, err
}

func (cs *CachedStorage) ModifyPost(ctx context.Context, userId string, postId string, newPost *post.Post) (*post.Post, error) {
	p, err := cs.InternalStorage.ModifyPost(ctx, userId, postId, newPost)
	if err != nil {
		return nil, err
	}
	cs.storeByPID(ctx, p)
	cs.findAndDeleteByUID(ctx, userId)
	return p, nil
}
