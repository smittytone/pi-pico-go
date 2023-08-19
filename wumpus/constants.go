package main

import (
	"machine"
)

/*
 * CONSTANTS
 */
const (
	// Game outcome strings
	textWin  string = "    YOU BEAT THE WUMPUS!    "
	textLose string = "    THE WUMPUS KILLED YOU!    "
	textFell string = "    YOU DIED!    "

	ON  bool = true
	OFF bool = false

	// GPIO pins
	PIN_SDA     machine.Pin = machine.GP8
	PIN_SCL     machine.Pin = machine.GP9
	PIN_GREEN   machine.Pin = machine.GP20
	PIN_RED     machine.Pin = machine.GP21
	PIN_SPEAKER machine.Pin = machine.GP16
	PIN_BUTTON  machine.Pin = machine.GP19

	// Joystick active range
	UPPER_LIMIT uint16 = 50000
	LOWER_LIMIT uint16 = 10000

	// Fire button debounce check timie
	DEBOUNCE_TIME_MS int64 = 10

	// Map markers
	PIT    uint8 = 'p'
	BAT    uint8 = 'b'
	WUMPUS uint8 = 'w'
	EMPTY  uint8 = '#'

	// Directions
	UP    uint = 0
	DOWN  uint = 1
	LEFT  uint = 2
	RIGHT uint = 3
	NONE  uint = 99

	PLAYER_PIXEL_FLASH_PERIOD_MS int64 = 200
)
