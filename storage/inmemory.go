package storage

import (
	"container/list"
	"context"
	"mini-twitter/domain/post"
	"mini-twitter/utils"
	"strconv"
	"strings"
	"sync"
)

const DEFAULT = -1

type InMemoryStorage struct {
	mu               sync.RWMutex
	Posts            *list.List
	PostIdToPost     map[string]*list.Element
	UserIdToPostsIds map[string][]string
	PostIdToIdx      map[string]int
}

func (im *InMemoryStorage) GetPostById(_ context.Context, postId string) (*post.Post, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	elem, ok := im.PostIdToPost[postId]
	if !ok {
		return nil, ErrPostNotFound
	}
	p := elem.Value.(*post.Post)
	return p, nil
}

func (im *InMemoryStorage) AddPost(_ context.Context, userId string, p *post.Post) {
	p.CreatedAt = utils.GetCurrentTimestamp()
	p.LastModifiedAt = utils.GetCurrentTimestamp()
	p.AuthorId = userId
	im.mu.Lock()
	defer im.mu.Unlock()
	for true {
		p.Id = utils.GeneratePostId()
		_, ok := im.PostIdToPost[p.Id]
		if !ok {
			break
		}
	}
	_, ok := im.UserIdToPostsIds[userId]
	if !ok {
		im.UserIdToPostsIds[userId] = make([]string, 0)
	}
	im.PostIdToIdx[p.Id] = len(im.UserIdToPostsIds[userId])
	im.UserIdToPostsIds[userId] = append(im.UserIdToPostsIds[userId], p.Id)
	im.Posts.PushBack(p)
	im.PostIdToPost[p.Id] = im.Posts.Back()
}

func (im *InMemoryStorage) GetPostsByUserId(_ context.Context, userId string, token string, size int) ([]*post.Post, string, error) {
	arr := make([]*post.Post, 0)
	im.mu.RLock()
	defer im.mu.RUnlock()
	_, ok := im.UserIdToPostsIds[userId]
	if !ok {
		if token != "" {
			return arr, "", ErrParseToken
		}
		return arr, "", nil
	}
	start := len(im.UserIdToPostsIds[userId]) - 1
	if token != "" {
		SizeAndPostId := strings.SplitN(token, "-", 2)
		if len(SizeAndPostId) != 2 {
			return arr, "", ErrParseToken
		}
		if size == DEFAULT {
			size, _ = strconv.Atoi(SizeAndPostId[0])
		}
		postId := SizeAndPostId[1]
		elem, ok := im.PostIdToPost[postId]
		p := elem.Value.(*post.Post)
		if !ok || p.AuthorId != userId {
			return arr, "", ErrParseToken
		}
		start = im.PostIdToIdx[postId] - 1
	}
	if size == DEFAULT {
		size = 10
	}
	end := start - size
	strSize := strconv.Itoa(size)
	retToken := ""
	if end >= 0 {
		retToken = strSize + "-" + im.UserIdToPostsIds[userId][end+1]
	} else {
		end = -1
	}
	for start > end {
		arr = append(arr, im.PostIdToPost[im.UserIdToPostsIds[userId][start]].Value.(*post.Post))
		start--
	}
	return arr, retToken, nil
}

func (im *InMemoryStorage) ModifyPost(_ context.Context, userId string, postId string, newPost *post.Post) (*post.Post, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	elem, ok := im.PostIdToPost[postId]
	if !ok {
		return nil, ErrPostNotFound
	}
	p := elem.Value.(*post.Post)
	if p.AuthorId != userId {
		return nil, ErrForbiddenAccess
	}
	p.Text = newPost.Text
	p.LastModifiedAt = utils.GetCurrentTimestamp()
	return p, nil
}
