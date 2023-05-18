package subscriptions

type Subscriptions struct {
	UserId        string   `bson:"user"`
	Subscriptions []string `bson:"subscriptions"`
}
