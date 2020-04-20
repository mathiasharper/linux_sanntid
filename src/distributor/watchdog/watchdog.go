package watchdog

import (
	. "../../elevatortypes"
	"time"
)

//Check if there are any hall orders
func hasOrders(globalState GlobalElevator) bool {
	for f := range globalState.HallRequests {
		for b := range globalState.HallRequests[f] {
			if globalState.HallRequests[f][b] {
				return true
			}
		}
	}
	return false
}

//InitWatchdog - Monitor that elevators are moving. if not, assign all hall orders to local elevator
func InitWatchdog(timeoutC chan<- bool, globalStateC <-chan GlobalElevator, timeout time.Duration) {

	floorMap := make(map[string]int)
	watchdogEnabled := false
	watchdogTimer := time.NewTimer(timeout)

	for {
		select {
		case newGlobalState := <-globalStateC:
			//Watchdog is enabled as long as there exists hall orders
			watchdogEnabled = hasOrders(newGlobalState)

			// Reset timer if an elevator has moved
			for newElevID, newElev := range newGlobalState.Elevators {
				if floor, ok := floorMap[newElevID]; ok {
					if floor != newElev.Floor {
						if watchdogTimer.Stop() {
							watchdogTimer.Reset(timeout)
						}
					}
				}
				floorMap[newElevID] = newElev.Floor
			}

		//Watchdog timed out, alert distributor
		case <-watchdogTimer.C:
			timeoutC <- true
			watchdogTimer.Reset(timeout)

		default:
			if !watchdogEnabled && watchdogTimer.Stop() {
				watchdogTimer.Reset(timeout)
			}
		}
	}
}
