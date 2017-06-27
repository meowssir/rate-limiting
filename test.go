package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/time/rate"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	minBSONSize = 4 + 1
	MaxBSONSize = 16 * 1024 * 1024 // 16MB - maximum BSON document size
)

type docQueue struct {
	opcount int
	docs    []interface{}
	idxs    []int
}

func archiveReader(filename string) (q docQueue) {
	r, _ := os.Open(filename)
	defer r.Close()

	// skip 0x8199e26d
	r.Seek(4, 0)
	buf := make([]byte, MaxBSONSize)

	out := bson.D{}
	for {
		// read size bytes
		n, err := io.ReadFull(r, buf[0:4])
		if n == 0 || err != nil {
			break
		}

		size := int32(
			(uint32(buf[0]) << 0) |
				(uint32(buf[1]) << 8) |
				(uint32(buf[2]) << 16) |
				(uint32(buf[3]) << 24),
		)

		if size != -1 {
			io.ReadFull(r, buf[4:size])
			bson.Unmarshal(buf, &out)
			q.docs = append(q.docs, out)
			q.opcount++
		}
	}
	return q
}

func limiter(r, b, opcount int, docs []interface{}, collection *mgo.Collection) {
	// if we buffer the channel to a value > 0 then the burst size will always be < b+(T*e).
	// https://en.wikipedia.org/wiki/Token_bucket
	c := make(chan int, 20)
	go func() {
		// create a Background context for incoming requests. It is never canceled, has no values, and has no deadline.
		ctx := context.Background()
		// converts a minimum time interval between events to a Limit, in this case n being 2.
		r := rate.Every(time.Second / time.Duration(r))
		// allows events up to rate r and permits bursts of at most b tokens.
		l := rate.NewLimiter(r, b)
		for i := 0; i < opcount; i++ {
			// If no token is available, Wait blocks until one can be obtained.
			if err := l.Wait(ctx); err != nil {
				log.Fatalln(err)
			}
			// send to channel when Wait unblocks and a token is made available.
			c <- i
		}
		close(c)
	}()
	op := opcount
	for r := range c {
		op--
		collection.Insert(docs[r])
		fmt.Println(r, time.Now(), op)
	}
}

func main() {

	// consumer queue and opcount manager
	q := docQueue{}
	q = archiveReader("dump")

	fmt.Println(q)
	fmt.Println(len(q.docs))

	// A token is added to the bucket every 1/r seconds.
	r := 4

	url := "ssp13g3l:27017"
	session, err := mgo.Dial(url)
	if err != nil {
		panic(err)
	}

	result := []bson.D{}
	c := session.DB("test").C("foo")

	// burst should be a configurable option based on what the user deems appropriate for policing rates.
	limiter(r, 5, q.opcount, q.docs, c)

	c.Find(bson.D{{}}).All(&result)
	fmt.Println(result)
}
