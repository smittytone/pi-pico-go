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
var hazards [8][8]uint8
var visited [8][8]bool
var stinkLayer [8][8]bool
var soundLayer [8][8]bool
var draughtLayer [8][8]bool

// Player state
var playerX uint
var playerY uint
var lastMoveDirection uint
var isInPlay bool
var isPlayerPixelOn bool

// Display instance
var matrix ht16k33.HT16K33

// Fire button debounce controls
var debounceButtonCount time.Time
var lastPlayerPixelFlash time.Time
var isJoystickCentred bool = true
var debounceButtonFlag bool = false

var PIN_Y machine.ADC = machine.ADC{Pin: machine.GP27}
var PIN_X machine.ADC = machine.ADC{Pin: machine.GP26}
