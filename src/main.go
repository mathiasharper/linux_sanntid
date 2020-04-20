package main

import(
    "./config"
    "./elevio"
    . "./elevatortypes"
    "./elevator"
    "./network/bcast"
    "./network/broadcaster"
    "./network/localip"
    "./distributor"
    "./distributor/watchdog"
    "./fsm"
    "time"
    "flag"
    "log"
    "strconv"
)
func main() {

	/* Local elevator id */
	var LOCAL_ID string
	if config.USE_LOCAL_IP_AS_ID {
		ip, err := localip.LocalIP()
        log.Println(ip)
		if err != nil {
			log.Panic("Main: Couldn't find IP!")
		}
		LOCAL_ID = ip
	} else {
		flag.StringVar(&LOCAL_ID, "id", "", "Local id for elevator")
		if LOCAL_ID == "" {
			log.Fatal("Main: No id input argument")
		}
	}

    
	/* Global State */
	globalState := elevator.GlobalElevatorInit(config.N_FLOORS, config.N_BUTTONS, LOCAL_ID)

	/* ocal hardware */
	localAddress := "localhost:" + strconv.Itoa(config.SERVER_PORT)
	elevio.Init(localAddress, config.N_FLOORS)
	buttonOrderC := make(chan ButtonEvent)
	floorEventC := make(chan int)
	go elevio.PollButtons(buttonOrderC)
	go elevio.PollFloorSensor(floorEventC)

	/* Door timer */
	doorTimerFinishedC := make(chan bool)
	doorTimerStartC := make(chan bool, 10) //Buffered to avoid blocking
	go fsm.InitDoorTimer(doorTimerFinishedC, doorTimerStartC, config.DOOR_OPEN_TIME)

	/* Local fsm */
	localStateUpdateC := make(chan Elevator)
	updateFSMRequestsC := make(chan [][]bool, 10)
	go fsm.InitFsm(
		localStateUpdateC,
		updateFSMRequestsC,
		floorEventC,
		doorTimerFinishedC,
		doorTimerStartC,
		LOCAL_ID,
		config.N_FLOORS,
		config.N_BUTTONS)

	/* Small hack to give local elevator time to reach a known state */
	t := time.Now()
	for time.Now().Sub(t) < 2*time.Second {
		select {
		case <-localStateUpdateC:
		default:
		}
	}

	/* Init network */
	networkTxC := make(chan GlobalElevator)
	networkRxC := make(chan GlobalElevator)
	go bcast.Transmitter(config.BROADCAST_PORT, networkTxC)
	go bcast.Receiver(config.BROADCAST_PORT, networkRxC)

	/* Init Broadcaster */
	updateBroadcastedPacketC := make(chan GlobalElevator, 10)
	networkStateUpdateC := make(chan GlobalElevator)
	elevatorLostEventC := make(chan string)
	go broadcaster.BroadcastState(updateBroadcastedPacketC, networkTxC, config.BCAST_INTERVAL)
	updateBroadcastedPacketC <- globalState
	go broadcaster.BroadcastListener(
		networkRxC,
		elevatorLostEventC,
		networkStateUpdateC,
		config.BROADCAST_TIMEOUT,
		LOCAL_ID,
		config.PACKET_LOSS_PERCENTAGE)

	/* Init watchdog */
	watchdogTimeoutC := make(chan bool)
	watchdogUpdateStateC := make(chan GlobalElevator, 10)
	go watchdog.InitWatchdog(watchdogTimeoutC, watchdogUpdateStateC, config.WATCHDOG_TIMEOUT)

	/* Merge state update channels from network and local FSM */
	stateUpdateC := make(chan GlobalElevator)
	go mergeUpdateChannels(networkStateUpdateC, localStateUpdateC, stateUpdateC)
	localStateUpdateC <- globalState.Elevators[LOCAL_ID]

	log.Println("Main: System initiated")
	distributor.RunDistributor(
		globalState,
		updateBroadcastedPacketC,
		stateUpdateC,
		elevatorLostEventC,
		watchdogTimeoutC,
		watchdogUpdateStateC,
		buttonOrderC,
		updateFSMRequestsC,
		config.N_FLOORS,
		config.N_BUTTONS,
		config.REASIGN_ON_CAB_ORDERS)
}

func mergeUpdateChannels(networkC <-chan GlobalElevator, fsmC <-chan Elevator, outC chan<- GlobalElevator) {

	localElev := <-fsmC
	globalDummy := elevator.GlobalElevatorInit(config.N_FLOORS, config.N_BUTTONS, localElev.ID)

	for {
		select {
		case update := <-networkC:
			outC <- update

		case globalDummy.Elevators[globalDummy.ID] = <-fsmC:
			outC <- globalDummy
		}
	}
}
