package main

import (
	"fmt"
	"io"
	"os"
	"time"

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

func main() {

	// consumer queue and opcount manager
	q := docQueue{}
	q = archiveReader("dump")

	fmt.Println(q)

	// n is the rate of inserts we want per second
	n := 2

	url := "ssp13g3l:27017"
	session, err := mgo.Dial(url)
	if err != nil {
		panic(err)
	}

	result := []bson.D{}
	c := session.DB("test").C("foo")

	requests := make(chan int, q.opcount)
	for i := 1; i < q.opcount; i++ {
		requests <- i
	}
	close(requests)

	// The actual throttle is defined by the tick time in ms as L/n where n is some int specified.
	// in the case of n being 2, the throttle is then 500ms.
	throttle := time.Tick(time.Millisecond * time.Duration(1000/n))

	for req := range requests {
		<-throttle // rate limit our inserts by blocking this channel.
		c.Insert(q.docs[req])
		fmt.Println(req, time.Now())
	}

	c.Find(bson.D{{}}).All(&result)
	fmt.Println(result)
}
