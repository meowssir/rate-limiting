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

func archiveReader(filename string) int32 {
	r, _ := os.Open(filename)

	b := make([]byte, 4)

	for {
		// skip 0x8199e26d and read size
		r.Seek(4, 0)
		r.Read(b)
		fmt.Println(b)
		size := int32(
			(uint32(b[0]) << 0) |
				(uint32(b[1]) << 8) |
				(uint32(b[2]) << 16) |
				(uint32(b[3]) << 24),
		)
		r.Seek(4, 0)

		buf := make([]byte, size)
		io.ReadFull(r, buf)
		out := bson.D{}
		bson.Unmarshal(buf, &out)
		fmt.Println(out)
		r.Seek(4, 0)
	}
}

func main() {

	archiveReader("dump")

	_ = "breakpoint"
	type docQueue struct {
		opcount int
		docs    []interface{}
		idxs    []int
	}

	q := docQueue{}
	q.opcount = 2
	q.docs = append(q.docs, bson.D{{"a", 0}}, bson.D{{"a", 1}})

	// n is the rate of inserts we want per second
	n := 2

	// queuing constructor and limit the number of exec per second.
	url := "ssp13g3l:27017"
	session, err := mgo.Dial(url)
	if err != nil {
		panic(err)
	}

	result := []bson.D{}
	c := session.DB("test").C("foo")
	c.Find(bson.D{{}}).All(&result)
	fmt.Println(result)
	fmt.Println(&q.docs)

	requests := make(chan int, q.opcount)
	for i := 1; i <= q.opcount; i++ {
		requests <- i
	}
	close(requests)

	// The actual throttle is defined by the tick time in ms as L/n where n is some int specified.
	// in the case of n being 2, the throttle is then 500ms.
	throttle := time.Tick(time.Millisecond * time.Duration(1000/n))

	for req := range requests {
		<-throttle // rate limit our inserts by blocking this channel.
		fmt.Println("request", req, time.Now())
	}
}
