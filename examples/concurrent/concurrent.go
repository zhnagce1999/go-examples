package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrCancelled -
var ErrCancelled = errors.New("cancel signal received")

// Strings -
type Strings <-chan string

// Each -
func (sch Strings) Each(ctx context.Context, workers int, fn func(int, string), doneFns ...func()) <-chan error {
	errCh := make(chan error, workers)

	go func() {
		defer close(errCh)

		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)

			go func(wid int) {
				defer wg.Done()
				for {
					// Why?
					select {
					case <-ctx.Done():
						errCh <- ErrCancelled
						return
					default:
					}

					select {
					case <-ctx.Done():
						errCh <- ErrCancelled
						return
					case item, hasMore := <-sch:
						if !hasMore {
							return
						}
						fn(wid, item)
					}
				}

			}(i)
		}

		wg.Wait()

		for _, doneFn := range doneFns {
			doneFn()
		}
	}()

	return errCh
}

// Map -
func (sch Strings) Map(ctx context.Context, workers int, mapFn func(int, string) string) (Strings, <-chan error) {
	outCh := make(chan string, workers)
	return outCh, sch.Each(ctx, workers, func(wid int, item string) {
		outCh <- mapFn(wid, item)
	}, func() {
		close(outCh)
	})
}

// Select -
func (sch Strings) Select(ctx context.Context, workers int, selectFn func(int, string) bool) (Strings, <-chan error) {
	outCh := make(chan string, workers)
	return outCh, sch.Each(ctx, workers, func(wid int, item string) {
		if selectFn(wid, item) {
			outCh <- item
		}
	}, func() {
		close(outCh)
	})
}

func generator(lines int) Strings {
	c := make(chan string)
	go func() {
		for i := 1; i < lines; i++ {
			var buf bytes.Buffer
			buf.WriteString(strconv.Itoa(i))
			for j := 0; j < i-1; j++ { // write i tab delimited values
				buf.WriteString("\t")
				buf.WriteString(strconv.Itoa(i))
			}
			c <- buf.String()
		}
		close(c)
	}()
	return c
}

func main() {
	ctx := context.Background()

	selected, selectErrors := generator(100).Select(ctx, 10, func(wid int, str string) bool {
		values := strings.Split(str, "\t")
		time.Sleep(50 * time.Millisecond)
		return len(values)%2 == 0
	})

	log.Println("This is printed before Select finishes because Select runs in the background")

	mapped, mapErrors := selected.Map(ctx, 10, func(wid int, str string) string {
		values := strings.Split(str, "\t")
		time.Sleep(50 * time.Millisecond)
		return strings.Join(values, ",")
	})

	for item := range mapped {
		log.Println(item)
	}

	// This is how we wait for Select to finish
	for err := range selectErrors {
		log.Println("There was an error:", err)
	}

	// This is how we wait for Map to finish
	for err := range mapErrors {
		log.Println("There was an error:", err)
	}
}
