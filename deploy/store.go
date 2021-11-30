package deploy

type Store interface {
	GetQueue(key string) Queue
	SetQueue(key string, q Queue)
	AddToHistory(key string, d Deploy)
}
