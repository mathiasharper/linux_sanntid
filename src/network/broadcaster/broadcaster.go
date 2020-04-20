package broadcaster

import (
	. "../../elevatortypes"
	"math/rand"
	"time"
)

func BroadcastListener(networkRx <-chan GlobalElevator,
	elevatorLostEventC chan<- string,
	stateUpdateC chan<- GlobalElevator,
	timeout time.Duration,
	localID string,
	packetLossPercentage float64) {

	lastSeen := make(map[string]time.Time)
	rand.Seed(42)

	for {
		select {
		case newPacket := <-networkRx:

			if newPacket.ID == localID {
				break
			}

			/* Simulate packet loss */
			if rand.Float64() < packetLossPercentage {
				break
			}

			lastSeen[newPacket.ID] = time.Now()
			stateUpdateC <- newPacket

		/* Generate lost elevator events */
		default:
			for id, t := range lastSeen {
				if time.Now().Sub(t) > timeout {
					elevatorLostEventC <- id
					delete(lastSeen, id)
				}
			}
		}
	}
}

func BroadcastState(updatePacketC <-chan GlobalElevator,
	toNetworkC chan<- GlobalElevator,
	interval time.Duration) {

	ticker := time.NewTicker(interval)

	latestPacket := <-updatePacketC

	for {
		select {
		case <-ticker.C:
			toNetworkC <- latestPacket
			//log.Println(latestPacket.HallRequests)

		case newPacket := <-updatePacketC:
			latestPacket = newPacket
		}
	}
}
