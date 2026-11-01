package subagent

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var sessionAdjectives = []string{"bold", "calm", "clever", "eager", "gentle", "lucky", "quiet", "swift"}
var sessionAnimals = []string{"badger", "falcon", "fox", "heron", "otter", "panda", "raven", "tiger"}

// GenerateSessionName returns a memorable name that is not present in taken.
func GenerateSessionName(taken func(string) bool) (string, error) {
	total := len(sessionAdjectives) * len(sessionAnimals)
	start, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return "", fmt.Errorf("generate session name: %w", err)
	}
	for offset := 0; offset < total; offset++ {
		i := (int(start.Int64()) + offset) % total
		name := sessionAdjectives[i/len(sessionAnimals)] + "-" + sessionAnimals[i%len(sessionAnimals)]
		if taken == nil || !taken(name) {
			return name, nil
		}
	}

	for suffix := 1; ; suffix++ {
		i := int(start.Int64()) % total
		name := fmt.Sprintf("%s-%s-%d", sessionAdjectives[i/len(sessionAnimals)], sessionAnimals[i%len(sessionAnimals)], suffix)
		if taken == nil || !taken(name) {
			return name, nil
		}
	}
}
