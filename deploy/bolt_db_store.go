package deploy

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

const (
	userIDKey       = "user.id"
	userNameKey     = "user.name"
	subjectKey      = "subject"
	startedAtKey    = "started_at"
	finishedAtKey   = "finished_at"
	abortedKey      = "aborted"
	pullRequestsKey = "prs"
	subscribersKey  = "subscribers"
)

var (
	ErrNoDeploy = errors.New("no deploys in channel")
)

type BoltDBStore struct {
	db *bolt.DB
}

func NewBoltDBStore(path string) (*BoltDBStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open db %s: %s", path, err)
	}

	return &BoltDBStore{db: db}, nil
}

func (s *BoltDBStore) GetQueue(key string) (queue Queue) {
	ok := false

	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(key))

		if bucket == nil {
			return nil
		}

		bytes := bucket.Get([]byte("queue"))

		if bytes == nil {
			return nil
		}

		err := json.Unmarshal(bytes, &queue)

		ok = err == nil

		return nil
	})

	if !ok {
		queue = NewEmptyQueue()
	}

	return queue
}

func (s *BoltDBStore) SetQueue(key string, queue Queue) {
	s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(key))

		if err != nil {
			return fmt.Errorf("failed to store queue %#v in channel %s: %s", queue, key, err)
		}

		bytes, err := json.Marshal(queue)

		if err != nil {
			return fmt.Errorf("failed to marshal queue %#v: %s", queue, err)
		}

		err = bucket.Put([]byte("queue"), bytes)

		if err != nil {
			return fmt.Errorf("failed to put queue into a bucket %#v: %s", queue, err)
		}

		return nil
	})
}

func (s *BoltDBStore) AddToHistory(key string, deploy Deploy) {
	s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(key))

		if err != nil {
			return fmt.Errorf("failed to create bucket for channel %s: %s", key, err)
		}

		bucket, err = bucket.CreateBucketIfNotExists([]byte("history"))

		if err != nil {
			return fmt.Errorf("failed to create bucket for channel history %s: %s", key, err)
		}

		bytes, err := json.Marshal(deploy)

		if err != nil {
			return fmt.Errorf("failed to marshal deploy %#v: %s", deploy, err)
		}

		// This returns an error only if the Tx is closed or not writeable.
		// That can't happen in an Update() call so we can ignore the error check.
		id, _ := bucket.NextSequence()

		err = bucket.Put(itob(id), bytes)

		if err != nil {
			return fmt.Errorf("failed to put deploy into a bucket %#v: %s", deploy, err)
		}

		return nil
	})
}

func (s *BoltDBStore) All(key string) []Deploy {
	var deploy Deploy
	var deploys []Deploy

	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(key))

		if bucket == nil {
			return nil
		}

		bucket = bucket.Bucket([]byte("history"))

		if bucket == nil {
			return nil
		}

		bucket.ForEach(func(k, v []byte) error {
			err := json.Unmarshal(v, &deploy)

			if err != nil {
				return nil
			}

			deploys = append(deploys, deploy)

			return nil
		})

		return nil
	})

	return deploys
}

func (s *BoltDBStore) Since(key string, startTime time.Time) []Deploy {
	var deploy Deploy
	var deploys []Deploy

	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(key))

		if bucket == nil {
			return nil
		}

		bucket = bucket.Bucket([]byte("history"))

		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			err := json.Unmarshal(v, &deploy)
			if err != nil {
				return err
			}

			if deploy.StartedAt.After(startTime) {
				deploys = append(deploys, deploy)
			}
		}

		return nil
	})

	return deploys
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}
