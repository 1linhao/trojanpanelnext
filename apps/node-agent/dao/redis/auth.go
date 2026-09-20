package redis

// AuthClient is deliberately limited to the read operation required for
// shared JWT and token material. It uses a separate Redis ACL identity from
// the cache client so write-capable commands can never target shared keys.
var AuthClient = new(authClient)

type authClient struct {
	String authStringRds
}

type authStringRds struct{}

func (authStringRds) Get(key string) *Reply {
	conn := authPool.Get()
	defer conn.Close()
	return getReply(conn.Do("get", key))
}
