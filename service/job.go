package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"pickup/app"
	"strconv"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/interfaces"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/activity"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/system"
	"github.com/Undercurrent-Technologies/kprime-utilities/repository/mongodb"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var isError bool = false

type JobService struct {
	system   interfaces.Repository[system.System]
	activity interfaces.Repository[activity.Activity]
}

func NewJobService(r *mongodb.Repositories) JobService {
	return JobService{system: r.System, activity: r.Activity}
}

func (js *JobService) NonceMonitoring() {
	// Default nonce difference
	nonceDiff, err := strconv.ParseFloat(app.Config.NonceDiff, 64)
	if err != nil {
		nonceDiff = 20
	}

	// Fetch nonce
	mongoNonce := js.fetchMongoNonce()
	engineNonce := js.fetchMatchingEngineNonce()

	// Fetch current system
	s := js.system.FindOne(bson.M{})
	now := time.Now()
	if s == nil {
		status := system.Status{Engine: types.ON, Gateway: types.ON}
		s = &system.System{ID: primitive.NewObjectID(), Status: status, CreatedAt: now, UpdatedAt: now}
		if _, err := js.system.Create(s); err != nil {
			return
		}
	}

	// Handle engine status
	if engineNonce == mongoNonce {
		// Start engine
		if s.Status.Engine == types.ON {
			return
		}

		s.Status.Engine = types.ON
	} else if math.Abs(engineNonce-mongoNonce) > nonceDiff {
		// Stop engine
		if s.Status.Engine == types.OFF {
			return
		}

		s.Status.Engine = types.OFF
	} else {
		// Doing nothing
		return
	}

	js.updateSystem(s, int(engineNonce), int(mongoNonce))
}

func (js *JobService) fetchMatchingEngineNonce() (nonce float64) {
	url := fmt.Sprintf("%s/api/v1/activities/nonce", app.Config.MatchingEngineURL)
	res, err := http.Get(url)
	if err != nil {
		if !isError {
			logs.Log.Error().Err(err).Msg("Matching engine is DISCONNECTED!")
			isError = true
		}
		return 0
	} else {
		if isError {
			logs.Log.Info().Msg("Matching engine is CONNECTED!")
			isError = false
		}
	}

	if res.StatusCode != 200 {
		logs.Log.Error().Msg(res.Status)
		return 0
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logs.Log.Error().Err(err).Msg("Failed to read data!")
		return 0
	}

	result := map[string]interface{}{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		logs.Log.Error().Err(err).Msg("Failed to decode data!")
		return 0
	}

	activity := result["data"].(map[string]interface{})
	return activity["data"].(float64)
}

func (js *JobService) fetchMongoNonce() (nonce float64) {
	pipeline := []bson.M{{"$sort": bson.M{"nonce": -1}}, {"$limit": 1}}
	activities, _ := js.activity.Aggregate(pipeline)
	if len(activities) > 0 {
		return float64(activities[0].Nonce)
	}

	return 0
}

func (js *JobService) updateSystem(s *system.System, en, mn int) {
	s.UpdatedAt = time.Now()

	filter := bson.M{"_id": s.ID}
	update := bson.M{"$set": s}

	if _, err := js.system.FindAndModify(filter, update); err == nil {
		msg := fmt.Sprintf("Matching engine is %s", s.Status.Engine.String())
		logs.Log.Info().Msg(msg)

		if s.Status.Engine == types.OFF {
			msg := fmt.Sprintf("Nonce is over %s", app.Config.NonceDiff)
			data := map[string]int{"engine": en, "mongo": mn}
			logs.Log.Info().Any("nonce", data).Msg(msg)
		}
	}
}
