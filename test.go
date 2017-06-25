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

func limiter(n, opcount int, docs []interface{}, intent *mgo.Collection) {
	// create a channel of an appropriate size for the operations.
	// TODO: investigate the performance of channels.
	c := make(chan int, 1000)
	go func() {
		// create a Background context for incoming requests. It is never canceled, has no values, and has no deadline.
		ctx := context.Background()
		// converts a minimum time interval between events to a Limit, in this case n being 2.
		r := rate.Every(time.Second / time.Duration(n))
		// allows events up to rate r and permits bursts of at most b tokens. 1 will not allow for bursts.
		// replace WaitN for a Wait and use the below formula for Burst size.
		// Given the ability to update the token bucket every S milliseconds, the number of tokens to add every S milliseconds
		// = (r*S)/1000.
		// Let M be the maximum possible transmission rate in bytes/second.
		// max(T) = b/(M-r) if r < M is the maximum burst time, that is the time for which the rate M is fully utilized.
		l := rate.NewLimiter(r, 1)
		for i := 0; i < opcount; i++ {
			if err := l.WaitN(ctx, 1); err != nil {
				log.Fatalln(err)
			}
			c <- i
		}
		close(c)
	}()
	for r := range c {
		intent.Insert(docs[r])
		fmt.Println(r, time.Now(), opcount)
	}
}

func main() {

	// consumer queue and opcount manager
	q := docQueue{}
	q = archiveReader("dump")

	fmt.Println(q)
	fmt.Println(len(q.docs))

	// n is the rate of inserts we want per second
	n := 2

	url := "ssp13g3l:27017"
	session, err := mgo.Dial(url)
	if err != nil {
		panic(err)
	}

	result := []bson.D{}
	c := session.DB("test").C("foo")

	limiter(n, q.opcount, q.docs, c)

	c.Find(bson.D{{}}).All(&result)
	fmt.Println(result)
}
