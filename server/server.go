package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	uuid "github.com/satori/go.uuid"
)

type MetricSummary struct {
	UniquePeriod int `json:"uniquePeriod"`
	TotalPeriod  int `json:"totalPeriod"`
}

type MetricMessage struct {
	Uuid              string
	ProtoVer          string
	DateTime          time.Time
	RubyVersion       string
	RubyPlatform      string
	CapistranoVersion string
	AnonProjectHash   string
}

func NewMetricMessage(wireLine string) *MetricMessage {
	s := strings.Split(wireLine, "|")
	dateTime, err := time.Parse(time.RFC3339Nano, s[1])
	if err != nil {
		log.Fatalf("Error parsing DateTime %s: %s\n", s[1], err)
		return nil
	}
	return &MetricMessage{
		Uuid:              uuid.NewV1().String(),
		ProtoVer:          s[0],
		DateTime:          dateTime,
		RubyVersion:       s[2],
		RubyPlatform:      s[3],
		CapistranoVersion: s[4],
		AnonProjectHash:   s[5],
	}
	return nil
}

func main() {

	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	udpAddress, err := net.ResolveUDPAddr("udp4", ":1200")

	if err != nil {
		fmt.Println("error resolving UDP address on ", ":1200")
		fmt.Println(err)
		return
	}

	conn, err := net.ListenUDP("udp", udpAddress)

	if err != nil {
		fmt.Println("error listening on UDP port ", ":1200")
		fmt.Println(err)
		return
	}

	defer conn.Close()

	var buf []byte = make([]byte, 1500)

	for {

		n, _, err := conn.ReadFromUDP(buf)

		if err != nil {
			log.Fatalln("Error", err)
			return
		}

		mm := NewMetricMessage(string(buf[0:n]))

		buckets := []string{
			fmt.Sprintf("%02d-%02d-%04d", mm.DateTime.Day(), mm.DateTime.Month(), mm.DateTime.Year()),
			fmt.Sprintf("%02d-%04d", mm.DateTime.Month(), mm.DateTime.Year()),
			fmt.Sprintf("%04d", mm.DateTime.Year()),
		}

		metrics := map[string]string{
			"anon_project_hash":  mm.AnonProjectHash,
			"capistrano_version": mm.CapistranoVersion,
			"proto_ver":          mm.ProtoVer,
			"ruby_platform":      mm.RubyPlatform,
			"ruby_version":       mm.RubyVersion,
		}

		for k, v := range metrics {
			_, err := c.Do("HSET", mm.Uuid, k, v)
			if err != nil {
				log.Fatalln(err)
			}
		}

		for _, bucket := range buckets {

			c.Do("SADD", fmt.Sprintf("%s", bucket), mm.Uuid)
			c.Do("SADD", fmt.Sprintf("%s|anon_project_hash", bucket), mm.AnonProjectHash)
			c.Do("SADD", fmt.Sprintf("%s|capistrano_versions", bucket), mm.CapistranoVersion)
			c.Do("SADD", fmt.Sprintf("%s|ruby_platforms", bucket), mm.RubyPlatform)
			c.Do("SADD", fmt.Sprintf("%s|ruby_versions", bucket), mm.RubyVersion)
			c.Do("SADD", fmt.Sprintf("%s|proto_versions", bucket), mm.ProtoVer)

		}

	}

}
