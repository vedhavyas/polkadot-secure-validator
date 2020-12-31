package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type bint struct {
	big.Int
}

func (b bint) MarshalJSON() ([]byte, error) {
	return []byte(b.String()), nil
}

type Metrics struct {
	NodeRoles int `json:"node_roles"`
	LibP2P    struct {
		PeerSetDiscovered int `json:"peer_set_discovered"`
		PeerSetRequested  int `json:"peer_set_requested"`
		Network           struct {
			In  *bint `json:"in"`
			Out *bint `json:"out"`
		} `json:"network"`
	} `json:"lib_p2p"`
	BlockHeight struct {
		Best       *bint `json:"best"`
		Finalized  *bint `json:"finalized"`
		SyncTarget *bint `json:"sync_target"`
	} `json:"block_height"`
	Peers          int            `json:"peers"`
	SyncPeers      int            `json:"sync_peers"`
	ForkTargets    int            `json:"fork_targets"`
	QueuedBlocks   int            `json:"queued_blocks"`
	IsMajorSyncing bool           `json:"is_major_syncing"`
	ValidatorStats ValidatorStats `json:"validator_stats"`
}

func fetchDataFromPrometheus() ([]byte, error) {
	u := "http://127.0.0.1:9615/metrics"
	var resp *http.Response
	var err error
	for i := 0; i < 5; i++ {
		resp, err = http.Get(u)
		if err != nil {
			log.Println("Failed. Will retry in a min again...")
			time.Sleep(time.Minute)
			continue
		}

		d, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println("Failed. Will retry in a min again...")
			log.Println(err)
			time.Sleep(time.Minute)
			continue
		}
		return d, nil
	}

	log.Println("Giving up. Notifying Admins...")
	return nil, errors.New("Failed to get metrics. Node maybe down!")
}

func parseData(data []byte) map[string]string {
	m := make(map[string]string)
	b := bufio.NewScanner(bytes.NewReader(data))
	for b.Scan() {
		if strings.HasPrefix(b.Text(), "#") {
			continue
		}

		splits := strings.Split(b.Text(), " ")
		m[splits[0]] = splits[1]
	}
	return m
}

func mustInt(d string) int {
	v, err := strconv.Atoi(d)
	if err != nil {
		panic(err)
	}

	return v
}

func mustBool(d string) bool {
	v, err := strconv.ParseBool(d)
	if err != nil {
		panic(err)
	}

	return v
}

func mustBigInt(s string) *bint {
	i := new(big.Int)
	i, ok := i.SetString(s, 10)
	if !ok {
		panic(fmt.Sprintf("invalid big int: %s", s))
	}

	return &bint{*i}
}

func FetchMetrics() (Metrics, error) {
	var metrics Metrics
	data, err := fetchDataFromPrometheus()
	if err != nil {
		return metrics, err
	}

	m := parseData(data)
	for k, v := range m {
		switch k {
		case "substrate_node_roles":
			metrics.NodeRoles = mustInt(v)
		case "substrate_sub_libp2p_network_bytes_total{direction=\"out\"}":
			metrics.LibP2P.Network.Out = mustBigInt(v)
		case "substrate_sub_libp2p_peerset_num_discovered":
			metrics.LibP2P.PeerSetDiscovered = mustInt(v)
		case "substrate_sub_libp2p_peerset_num_requested":
			metrics.LibP2P.PeerSetRequested = mustInt(v)
		case "substrate_sync_fork_targets":
			metrics.ForkTargets = mustInt(v)
		case "substrate_sync_peers":
			metrics.SyncPeers = mustInt(v)
		case "substrate_sync_queued_blocks":
			metrics.QueuedBlocks = mustInt(v)
		case "substrate_block_height{status=\"best\"}":
			metrics.BlockHeight.Best = mustBigInt(v)
		case "substrate_block_height{status=\"sync_target\"}":
			metrics.BlockHeight.SyncTarget = mustBigInt(v)
		case "substrate_sub_libp2p_is_major_syncing":
			metrics.IsMajorSyncing = mustBool(v)
		case "substrate_sub_libp2p_network_bytes_total{direction=\"in\"}":
			metrics.LibP2P.Network.In = mustBigInt(v)
		case "substrate_block_height{status=\"finalized\"}":
			metrics.BlockHeight.Finalized = mustBigInt(v)
		case "substrate_sub_libp2p_peers_count":
			metrics.Peers = mustInt(v)
		}
	}

	vs, err := fetchValidatorStats()
	if err != nil {
		return metrics, err
	}
	metrics.ValidatorStats = vs
	return metrics, nil
}

func (m Metrics) String() string {
	d, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(err)
	}

	var buf strings.Builder
	buf.WriteString("```\n")
	buf.WriteString(string(d))
	buf.WriteString("\n```")
	return buf.String()
}
