package subscribers

type Subscribers struct {
	UserId      string   `bson:"user"`
	Subscribers []string `bson:"subscribers"`
}
