package main

import (
	"log"
	"os"
	"sync"
	"time"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/garyburd/redigo/redis"

)
import rediscluster "github.com/korvus81/redis-go-cluster"

type Task struct {
	list []string
}

type Worker func(queue chan Task, wg *sync.WaitGroup)

type Config struct {
	Dest       string
	Source     string
	DestPass   string
	SourcePass string
	Workers    int
	Batch      int
	Prefix     string
	ClearDest  bool
	DryRun     bool
}

var config Config

func main() {
	app := cli.NewApp()
	app.Name = "migr8"
	app.Usage = "It's time to move some redis"
	app.Commands = []cli.Command{
		{
			Name:   "migrate",
			Usage:  "Migrate one redis to a new redis",
			Action: Migrate,
		},
		{
			Name:   "delete",
			Usage:  "Delete all keys with the given prefix",
			Action: Delete,
		},
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "dry-run, n",
			Usage: "Run in dry-run mode",
		},
		cli.StringFlag{
			Name:  "source, s",
			Usage: "The redis server to pull data from",
			Value: "127.0.0.1:6379",
		},
		cli.StringFlag{
			Name:  "dest, d",
			Usage: "The destination redis server",
			Value: "127.0.0.1:6379",
		},
		cli.StringFlag{
			Name: "destpass",
			Usage: "The password for the destination redis server",
			Value: "",
		},
		cli.StringFlag{
			Name: "sourcepass",
			Usage: "The password for the source redis server",
			Value: "",
		},
		cli.IntFlag{
			Name:  "workers, w",
			Usage: "The count of workers to spin up",
			Value: 2,
		},
		cli.IntFlag{
			Name:  "batch, b",
			Usage: "The batch size",
			Value: 10,
		},
		cli.StringFlag{
			Name:  "prefix, p",
			Usage: "The key prefix to act on",
		},
		cli.BoolFlag{
			Name:  "clear-dest, c",
			Usage: "Clear the destination of all it's keys and values",
		},
	}

	app.Run(os.Args)
}

func ParseConfig(c *cli.Context) {
	config = Config{
		Source:    c.GlobalString("source"),
		SourcePass:c.GlobalString("sourcepass"),
		Dest:      c.GlobalString("dest"),
		DestPass:  c.GlobalString("destpass"),
		Workers:   c.GlobalInt("workers"),
		Batch:     c.GlobalInt("batch"),
		Prefix:    c.GlobalString("prefix"),
		ClearDest: c.GlobalBool("clear-dest"),
		DryRun:    c.GlobalBool("dry-run"),
	}
}

func sourceConnection(source string, auth string) redis.Conn {
	// attempt to connect to source server
	sourceConn, err := redis.Dial("tcp", source)
	if err != nil {
		panic(err)
	}
	if auth != "" {
		if _, err := sourceConn.Do("AUTH",auth); err != nil {
			panic(err)
		}
	}
	return sourceConn
}

func destConnection(dest string, auth string) *rediscluster.Cluster {
	// attempt to connect to source server
	/*destConn, err := redis.Dial("tcp", dest)
	if err != nil {
		panic(err)
	}

	return destConn*/
	destConn, err := rediscluster.NewCluster(
		&rediscluster.Options{
			StartNodes: strings.Split(dest,","),//[]string{dest},
			Password: auth,
			ConnTimeout: 5000 * time.Millisecond,
			ReadTimeout: 100 * time.Millisecond,
			WriteTimeout: 100 * time.Millisecond,
			KeepAlive: 16,
			AliveTime: 60 * time.Second,
		})
	if err != nil {
		panic(err)
	}
	return destConn
}

func RunAction(action Worker) {
	wg := &sync.WaitGroup{}
	workQueue := make(chan Task, config.Workers)
	startedAt = time.Now()

	wg.Add(1)
	go scanKeys(workQueue, wg)

	for i := 0; i <= config.Workers; i++ {
		wg.Add(1)
		go action(workQueue, wg)
	}

	wg.Wait()
}

func Migrate(c *cli.Context) {
	ParseConfig(c)
	log.Printf("Running migrate with config: %+v\n", config)
	log.SetPrefix("migrate - ")

	if config.ClearDest {
		clearDestination(c.String("dest"), c.String("destpass"))
	}

	RunAction(migrateKeys)
}

func Delete(c *cli.Context) {
	ParseConfig(c)
	log.Printf("Running delete with config: %+v\n", config)
	log.SetPrefix("delete - ")

	RunAction(deleteKeys)
}
