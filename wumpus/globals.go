/*
 * Hunt the Wumpus for Raspberry Pi Pico
 * Go version
 *
 * @version     1.0.2
 * @authors     smittytone
 * @copyright   2023, Tony Smith
 * @licence     MIT
 *
 */
 package main

import (
	"machine"
	"time"
	"wumpus/ht16k33"
)

/*
 * GLOBALS
 */
// Game board maps
var (
	hazards [8][8]uint8
	visited [8][8]bool
	stinkLayer [8][8]bool
	soundLayer [8][8]bool
	draughtLayer [8][8]bool

	// Player state
	playerX uint
	playerY uint
	lastMoveDirection uint
	isInPlay bool
	isPlayerPixelOn bool

	// Display instance
	matrix ht16k33.HT16K33

	// Fire button debounce controls
	debounceButtonCount time.Time
	lastPlayerPixelFlash time.Time
	isJoystickCentred bool = true
	debounceButtonFlag bool = false

	PIN_Y machine.ADC = machine.ADC{Pin: machine.GP27}
	PIN_X machine.ADC = machine.ADC{Pin: machine.GP26}

	// FROM 1.0.1
	gamesWon uint
	gamesLost uint
)
