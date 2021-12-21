package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"
	geo "github.com/synerex/proto_geography"
	pagent "github.com/synerex/proto_people_agent"
	pb "github.com/synerex/synerex_api"
	pbase "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"
)

var (
	nodesrv = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	channel = flag.String("channel", "13,14", "pbase.GEOGRAPHIC_SVC,pbase.PEOPLE_AGENT_SVC is also OK")
	local   = flag.String("local", "", "Specify Local Synerex Server")
	sendnum = flag.Int("sendnum", 1, "1:lines,2:mesh,3:agent")
	// 名古屋市役所の緯度経度
	nlat = 35.181433
	nlon = 136.906421
	// 名古屋大学の緯度経度
	nulat = 35.153760239787
	nulon = 136.96418918068
	// 東山公園の緯度経度
	hlon = 136.9790
	hlat = 35.1567
)

func init() {

}

func sendLines(clients map[uint32]*sxutil.SXServiceClient) {

	l1 := &geo.Line{
		From: []float64{nlon, nlat},
		To:   []float64{nulon, nulat},
	}
	l2 := &geo.Line{
		From: []float64{nulon, nulat},
		To:   []float64{hlon, hlat},
	}
	l3 := &geo.Line{
		From: []float64{nlon, nlat},
		To:   []float64{hlon, hlat},
	}
	l123 := geo.Lines{
		Lines: []*geo.Line{l1, l2, l3},
		Width: 100,
		Color: []int32{0, 0, 0},
	}

	out, _ := proto.Marshal(&l123)
	cont := pb.Content{Entity: out}
	spo := sxutil.SupplyOpts{
		Name:  "Lines",
		Cdata: &cont,
	}
	// if channel in channels
	chnum, err := strconv.Atoi("14")
	client, ok := clients[uint32(chnum)]
	if ok && err == nil { // if there is channel
		time.Sleep(1 * time.Second)
		_, nerr := client.NotifySupply(&spo)
		if nerr != nil {
			log.Printf("Send Fail!%v", nerr)
		}
	}
}

func sendMesh(clients map[uint32]*sxutil.SXServiceClient) {

	type MeshItem struct {
		Position []float64 `json:"pos"`
		Value    float64   `json:"val"`
		Color    []int     `json:"col"`
	}

	type MeshBlock struct {
		ID int        `json:"id"`
		Ts int64      `json:"timestamp"`
		Ms []MeshItem `json:"meshItems"`
	}
	type MeshJson struct {
		Mblock MeshBlock `json:"addMesh"`
	}
	now := time.Now()
	size := 0.004
	xw := 15
	yw := 10
	for clock := 0; clock < 32; clock++ {

		mitem := []MeshItem{}
		for x := -xw; x < xw; x++ {
			for y := -yw; y <= yw; y++ {
				v := x * y
				if v < 0 {
					v = -v
				}
				r := 5000 / (v + clock + 1)
				pos := []float64{hlon + float64(x)*size, hlat + float64(y)*size}
				mitem = append(mitem, MeshItem{Position: pos, Value: float64(v + clock*10), Color: []int{int(r), 35, 142, 100}})
			}
		}
		t := now.Add(time.Duration(clock) * time.Second).Unix()
		mb := MeshBlock{ID: 1000000, Ts: t, Ms: mitem}
		mjson := &MeshJson{Mblock: mb}
		outputJson, err := json.Marshal(mjson)
		if err != nil {
			panic(err)
		}
		msg := geo.HarmoVIS{
			ConfJson: string(outputJson),
		}
		out, _ := proto.Marshal(&msg)
		cont := pb.Content{Entity: out}
		spo := sxutil.SupplyOpts{
			Name:  "HarmoVIS",
			Cdata: &cont,
		}

		client, ok := clients[uint32(pbase.GEOGRAPHIC_SVC)]
		if ok == true { // if there is channel
			_, nerr := client.NotifySupply(&spo)
			if nerr != nil {
				log.Printf("Send Fail!%v", nerr)
			}
		}
	}

}

func sendPAgent(clients map[uint32]*sxutil.SXServiceClient) {

	pa1from := &pagent.PAgent{
		Id:    1,
		Point: []float64{nlon, nlat},
	}
	pa2from := &pagent.PAgent{
		Id:    2,
		Point: []float64{nulon, nulat},
	}
	pa1to := &pagent.PAgent{
		Id:    1,
		Point: []float64{nulon, nulat},
	}
	pa2to := &pagent.PAgent{
		Id:    2,
		Point: []float64{nlon, nlat},
	}
	msg1 := pagent.PAgents{
		Agents: []*pagent.PAgent{pa1from, pa2from},
	}
	msg2 := pagent.PAgents{
		Agents: []*pagent.PAgent{pa1to, pa2to},
	}

	out1, _ := proto.Marshal(&msg1)
	cont1 := pb.Content{Entity: out1}
	spo1 := sxutil.SupplyOpts{
		Name:  "Agents",
		Cdata: &cont1,
	}

	out2, _ := proto.Marshal(&msg2)
	cont2 := pb.Content{Entity: out2}
	spo2 := sxutil.SupplyOpts{
		Name:  "Agents",
		Cdata: &cont2,
	}

	client, ok := clients[uint32(pbase.PEOPLE_AGENT_SVC)]
	if ok == true { // if there is channel
		_, nerr := client.NotifySupply(&spo1)
		if nerr != nil {
			log.Printf("Send Fail!%v", nerr)
		}
		time.Sleep(5 * time.Second)
		_, nerr = client.NotifySupply(&spo2)
		if nerr != nil {
			log.Printf("Send Fail!%v", nerr)
		}
	}
}

func main() {
	log.Printf("harmoVIS_sample(%s) built %s sha1 %s", sxutil.GitVer, sxutil.BuildTime, sxutil.Sha1Ver)
	flag.Parse()
	go sxutil.HandleSigInt()
	sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)

	// check channel types.
	//
	channelTypes := []uint32{}
	chans := strings.Split(*channel, ",")
	for _, ch := range chans {
		v, err := strconv.Atoi(ch)
		if err == nil {
			channelTypes = append(channelTypes, uint32(v))
		} else {
			log.Fatal("Can't convert channels ", *channel)
		}
	}

	srv, rerr := sxutil.RegisterNode(*nodesrv, fmt.Sprintf("harmoVIS_sample[%s]", *channel), channelTypes, nil)

	if rerr != nil {
		log.Fatal("Can't register node:", rerr)
	}
	if *local != "" { // quick hack for AWS local network
		srv = *local
	}
	log.Printf("Connecting SynerexServer at [%s]", srv)

	//	wg := sync.WaitGroup{} // for syncing other goroutines

	client := sxutil.GrpcConnectServer(srv)

	if client == nil {
		log.Fatal("Can't connect Synerex Server")
	} else {
		log.Print("Connecting SynerexServer")
	}

	// we need to add clients for each channel:
	clients := map[uint32]*sxutil.SXServiceClient{}

	for _, chnum := range channelTypes {
		argJson := fmt.Sprintf("{harmoVIS_sample[%d]}", chnum)
		clients[chnum] = sxutil.NewSXServiceClient(client, chnum, argJson)
	}
	if *sendnum == 1 {
		sendLines(clients)
	} else if *sendnum == 2 {
		sendMesh(clients)
	} else if *sendnum == 3 {
		sendPAgent(clients)
	}

}
