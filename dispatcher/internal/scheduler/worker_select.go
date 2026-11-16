package scheduler

import (
	"math/rand"
	"sort"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

// chooseWorker orders candidates by priority, then lowest running count, then random.
func chooseWorker(candidates []*config.WorkerConfig, runningCounts map[string]int) []*config.WorkerConfig {
	shuffled := append([]*config.WorkerConfig(nil), candidates...)
	rand.Shuffle(len(shuffled), func(left, right int) {
		shuffled[left], shuffled[right] = shuffled[right], shuffled[left]
	})
	sort.SliceStable(shuffled, func(left, right int) bool {
		if shuffled[left].Priority != shuffled[right].Priority {
			return shuffled[left].Priority < shuffled[right].Priority
		}
		return runningCounts[shuffled[left].Name] < runningCounts[shuffled[right].Name]
	})
	return shuffled
}
