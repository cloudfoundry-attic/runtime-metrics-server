package instruments

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cloudfoundry-incubator/cf_http"
	"github.com/cloudfoundry-incubator/runtime-schema/metric"
	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/pivotal-golang/lager"
)

var errRedirected = errors.New("redirected to leader")

const (
	etcdLeader                = metric.Metric("ETCDLeader")
	etcdFollowers             = metric.Metric("ETCDFollowers")
	etcdReceivedBandwidthRate = metric.BytesPerSecond("ETCDReceivedBandwidthRate")
	etcdSentBandwidthRate     = metric.BytesPerSecond("ETCDSentBandwidthRate")
	etcdReceivedRequestRate   = metric.RequestsPerSecond("ETCDReceivedRequestRate")
	etcdSentRequestRate       = metric.RequestsPerSecond("ETCDSentRequestRate")
	etcdRaftTerm              = metric.Metric("ETCDRaftTerm")
	etcdWatchers              = metric.Metric("ETCDWatchers")
)

type etcdInstrument struct {
	logger lager.Logger

	etcdCluster []string

	client *http.Client
}

func NewETCDInstrument(logger lager.Logger, etcdCluster []string) Instrument {
	client := cf_http.NewClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errRedirected
	}

	return &etcdInstrument{
		logger: logger,

		etcdCluster: etcdCluster,

		client: client,
	}
}

func (t *etcdInstrument) Send() {
	for i, etcdAddr := range t.etcdCluster {
		t.sendLeaderStats(etcdAddr, i)
		// newEtcdNodeStats(etcdAddr, i, t.logger).Send()
	}

	t.sendSelfStats()
}

func (t *etcdInstrument) sendLeaderStats(etcdAddr string, index int) {
	resp, err := t.client.Get(t.leaderStatsEndpoint(etcdAddr))
	if err != nil {
		t.logger.Error("failed-to-collect-stats", err)
		return
	}

	defer resp.Body.Close()

	var stats etcdLeaderStats

	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		t.logger.Error("failed-to-unmarshal-stats", err)
		return
	}

	etcdLeader.Send(index)

	var storeStats etcdStoreStats

	resp, err = t.client.Get(t.storeStatsEndpoint(etcdAddr))
	if err != nil {
		t.logger.Error("failed-to-collect-stats", err)
		return
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&storeStats)
	if err != nil {
		t.logger.Error("failed-to-unmarshal-stats", err)
		return
	}

	resp, err = t.client.Get(t.keysEndpoint((etcdAddr)))
	if err != nil {
		t.logger.Error("failed-to-get-keys", err)
		return
	}

	resp.Body.Close()

	raftTermHeader := resp.Header.Get("X-Raft-Term")

	raftTerm, err := strconv.ParseInt(raftTermHeader, 10, 0)
	if err != nil {
		t.logger.Error("failed-to-parse-raft-term", err, lager.Data{
			"term": raftTermHeader,
		})
		return
	}

	etcdRaftTerm.Send(int(raftTerm))
	etcdWatchers.Send(int(storeStats.Watchers))

	return
}

func (t *etcdInstrument) sendSelfStats() {
	var receivedRequestsPerSecond float64
	var sentRequestsPerSecond float64

	var receivedBandwidthRate float64
	var sentBandwidthRate float64

	for _, addr := range t.etcdCluster {
		var selfStats etcdServerStats

		resp, err := t.client.Get(t.selfStatsEndpoint(addr))
		if err != nil {
			t.logger.Error("failed-to-collect-stats", err)
			return
		}

		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&selfStats)
		if err != nil {
			t.logger.Error("failed-to-unmarshal-stats", err)
			return
		}

		if selfStats.RecvingPkgRate != nil {
			receivedRequestsPerSecond += *selfStats.RecvingPkgRate
		}

		if selfStats.RecvingBandwidthRate != nil {
			receivedBandwidthRate += *selfStats.RecvingBandwidthRate
		}

		if selfStats.SendingPkgRate != nil {
			sentRequestsPerSecond += *selfStats.SendingPkgRate
		}

		if selfStats.SendingBandwidthRate != nil {
			sentBandwidthRate += *selfStats.SendingBandwidthRate
		}
	}

	etcdSentBandwidthRate.Send(sentBandwidthRate)
	etcdSentRequestRate.Send(float64(sentRequestsPerSecond))

	etcdReceivedBandwidthRate.Send(receivedBandwidthRate)
	etcdReceivedRequestRate.Send(float64(receivedRequestsPerSecond))
}

func (t *etcdInstrument) leaderStatsEndpoint(etcdAddr string) string {
	return urljoiner.Join(etcdAddr, "v2", "stats", "leader")
}

func (t *etcdInstrument) selfStatsEndpoint(etcdAddr string) string {
	return urljoiner.Join(etcdAddr, "v2", "stats", "self")
}

func (t *etcdInstrument) storeStatsEndpoint(etcdAddr string) string {
	return urljoiner.Join(etcdAddr, "v2", "stats", "store")
}

func (t *etcdInstrument) keysEndpoint(etcdAddr string) string {
	return urljoiner.Join(etcdAddr, "v2", "keys")
}

type etcdLeaderStats struct {
	Leader    string `json:"leader"`
	Followers map[string]struct {
		Latency struct {
			Current           float64 `json:"current"`
			Average           float64 `json:"average"`
			averageSquare     float64
			StandardDeviation float64 `json:"standardDeviation"`
			Minimum           float64 `json:"minimum"`
			Maximum           float64 `json:"maximum"`
		} `json:"latency"`

		Counts struct {
			Fail    uint64 `json:"fail"`
			Success uint64 `json:"success"`
		} `json:"counts"`
	} `json:"followers"`
}

type etcdServerStats struct {
	Name  string `json:"name"`
	State string `json:"state"`

	LeaderInfo struct {
		Name   string `json:"leader"`
		Uptime string `json:"uptime"`
	} `json:"leaderInfo"`

	RecvAppendRequestCnt uint64   `json:"recvAppendRequestCnt,"`
	RecvingPkgRate       *float64 `json:"recvPkgRate,omitempty"`
	RecvingBandwidthRate *float64 `json:"recvBandwidthRate,omitempty"`

	SendAppendRequestCnt uint64   `json:"sendAppendRequestCnt"`
	SendingPkgRate       *float64 `json:"sendPkgRate,omitempty"`
	SendingBandwidthRate *float64 `json:"sendBandwidthRate,omitempty"`
}

type etcdStoreStats struct {
	Watchers uint64 `json:"watchers"`
}
