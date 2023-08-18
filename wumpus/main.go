/*
 * Hunt the Wumpus for Raspberry Pi Pico
 * Go version
 *
 * @version     0.1.0
 * @authors     smittytone
 * @copyright   2023, Tony Smith
 * @licence     MIT
 *
 */
package main

import (
	//"crypto/rand"
	"machine"
	prand "math/rand"
	"time"
	"wumpus/ht16k33"
	"wumpus/graphics"
	//"fmt"
)

/*
 *  Globals
 */
// Wumpus World
var hazards [8][8]uint8
var visited [8][8]bool
var stink_layer [8][8]bool
var sound_layer [8][8]bool
var draught_layer [8][8]bool

var player_x uint
var player_y uint
var lastMoveDirection uint8

var isInPlay bool
var isPlayerPixelOn bool

const textWin string = "    YOU WIN!    "
const textLose string = "    YOU DIED!    "

// I2C bus
//var i2c machine.I2C
var matrix ht16k33.HT16K33

// Debounce controls
var debounceButtonCount time.Time
var lastPlayerPixelFlash time.Time
var isJoystickCentred bool = true
var debounceButtonFlag bool = false

const ON bool = true
const OFF bool = false

const PIN_SDA machine.Pin = machine.GP8
const PIN_SCL machine.Pin = machine.GP9

const PIN_GREEN machine.Pin = machine.GP20
const PIN_RED machine.Pin = machine.GP21
const PIN_SPEAKER machine.Pin = machine.GP16

var PIN_Y machine.ADC = machine.ADC{Pin: machine.GP27}
var PIN_X machine.ADC = machine.ADC{Pin: machine.GP26}

const PIN_BUTTON machine.Pin = machine.GP19

const DEADZONE uint16 = 400
const UPPER_LIMIT uint16 = 50000
const LOWER_LIMIT uint16 = 10000

const DEBOUNCE_TIME_MS int64 = 10

func main() {

	// Set up the hardware or fail
	if !setup() {
		failLoop()
	}

	// Play the game
	for {
		// Set up a new round...
		// Play the Wumpus tune
		playIntro()

		// Set up the environment
		createWorld()
		drawWorld()
		checkSenses()

		// ...and start play
		gameLoop()
	}

	return
}

/*
 *  Initialisation Functions
 */
func setup() bool {
	// Set up the game hardware
	i2c := machine.I2C0
	err := i2c.Configure(machine.I2CConfig{SCL: PIN_SCL, SDA: PIN_SDA})
	if err != nil {
		// Couldn't configure I2C
		return false
	}

	// Set up the LED matrix
	matrix = ht16k33.New(*i2c)
	matrix.Init()

	// Set up sense indicator output pins:
	// Green is the Wumpus nearby indicator
	PIN_GREEN.Configure(machine.PinConfig{Mode: machine.PinOutput})
	PIN_GREEN.Low()

	// Red is the Pit nearby indicator
	PIN_RED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	PIN_RED.Low()

	// Set up the speaker
	PIN_SPEAKER.Configure(machine.PinConfig{Mode: machine.PinOutput})
	PIN_SPEAKER.Low()

	// Set up the Fire button
	PIN_BUTTON.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})

	// Set up the X- and Y-axis joystick input
	machine.InitADC()
	err = PIN_X.Configure(machine.ADCConfig{})
	if err != nil {
		return false
	}
	err = PIN_Y.Configure(machine.ADCConfig{})
	if err != nil {
		return false
	}
	
	return true
}

func createWorld() {

	// Generate the Wumpus' cave

	// The player starts at (0,0)
	player_x = 0
	player_y = 0
	isInPlay = true

	// Zero the world arrays
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			hazards[i][j] = '#' // No hazard
			visited[i][j] = false
			stink_layer[i][j] = false
			draught_layer[i][j] = false
			sound_layer[i][j] = false
		}
	}

	// Create 1-3 bats
	rollHazards('b')

	// Create 1-3 pits
	rollHazards('p')

	// Create one wumpus
	// NOTE It's generated last so bats and pits
	//      can't overwrite it by chance, and we
	//      make sure it's not in the bottom left
	//      corner
	var wumpus_x uint = 7
	var wumpus_y uint = 7
	
	for wumpus_x < 1 && wumpus_y < 1 {
		wumpus_x = irandom(0, 8)
		wumpus_y = irandom(0, 8)
	}

	// Set its location
	hazards[wumpus_x][wumpus_y] = 'w'

	// Make sure the start tile is safe to spawn on
	hazards[0][0] = '#'

	// Generate sense data for sounds and LED reactions
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			if hazards[i][j] == 'w' {
				if i < 7 {
					stink_layer[i+1][j] = true
				}
				if i > 0 {
					stink_layer[i-1][j] = true
				}
				if j < 7 {
					stink_layer[i][j+1] = true
				}
				if j > 0 {
					stink_layer[i][j-1] = true
				}
			} else if hazards[i][j] == 'p' {
				if i < 7 {
					draught_layer[i+1][j] = true
				}
				if i > 0 {
					draught_layer[i-1][j] = true
				}
				if j < 7 {
					draught_layer[i][j+1] = true
				}
				if j > 0 {
					draught_layer[i][j-1] = true
				}
			} else if hazards[i][j] == 'b' {
				if i < 7 {
					sound_layer[i+1][j] = true
				}
				if i > 0 {
					sound_layer[i-1][j] = true
				}
				if j < 7 {
					sound_layer[i][j+1] = true
				}
				if j > 0 {
					sound_layer[i][j-1] = true
				}
			}
		}
	}
}

func rollHazards(hazardType uint8) {

	var hazard_x uint = 0
	var hazard_y uint = 0
	var count = irandom(1, 4)
	var i uint
	for i = 0; i < count; i++ {
		hazard_x = irandom(0, 8)
		hazard_y = irandom(0, 8)
		hazards[hazard_x][hazard_y] = hazardType
	}
}

/*
 *  Main Game Loop
 */
func gameLoop() {
	// Read the current joystick position.
	// If it's not in the deadzone, then determine
	// which direction it's in (up, down, left or right).
	// If it's in the deadzone, check if the player is trying
	// to fire an arrow.
	
	isInPlay = true
	debounceButtonFlag = false
	for isInPlay {
		// Read joystick analog output
		x := PIN_X.Get()
		y := PIN_Y.Get()
		isDead := false
		//fmt.Printf("X: %d, Y: %d\r\n", x ,y)
	
		if checkJoystick(x, y) {
			// Joystick is pointing in a direction, so
			// get the direction the player has chosen
			direction := getDirection(x, y)

			// Record the player's steps before the move
			visited[player_x][player_y] = true

			if direction == 0 {
				// Move player up
				if player_y < 7 {
					player_y += 1
					lastMoveDirection = 0
				}
			} else if direction == 3 {
				// Move player right
				if player_x < 7 {
					player_x += 1
					lastMoveDirection = 3
				}
			} else if direction == 2 {
				// Move player down
				if player_y > 0 {
					player_y -= 1
					lastMoveDirection = 2
				}
			} else {
				// Move player left
				if player_x > 0 {
					player_x -= 1
					lastMoveDirection = 1
				}
			}

			// Check the new location for sense
			// information and hazards
			isDead = checkHazards()
			if !isDead {
				checkSenses()
			}
		} else {
			// Joystick is in deadzone so can fire
			if PIN_BUTTON.Get() {
				if !debounceButtonFlag {
					// Set debounce timer
					debounceButtonCount = time.Now()
					debounceButtonFlag = true
				} else if time.Since(debounceButtonCount).Milliseconds() > DEBOUNCE_TIME_MS {
					// Clear debounce timer
					debounceButtonFlag = false

					// Shoot arrow
					fireArrowAnimation()

					// Did the arrow hit or miss?
					if lastMoveDirection == 0 {
						if player_y < 7 {
							if hazards[player_x][player_y+1] == 'w' {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					} else if lastMoveDirection == 3 {
						if player_x < 7 {
							if hazards[player_x+1][player_y] == 'w' {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					} else if lastMoveDirection == 2 {
						if player_y > 0 {
							if hazards[player_x][player_y-1] == 'w' {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					} else {
						if player_x > 0 {
							if hazards[player_x-1][player_y] == 'w' {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					}
				}
			}
		}

		matrix.Plot(7,7,isJoystickCentred)

		if !isDead {
			// Draw the world then check for smells and hazards
			drawWorld()

			// Pause between cycles
			time.Sleep(50 * time.Millisecond)
		}
	}
}

/*
 *  Movement control functions
 */
func checkJoystick(x uint16, y uint16) bool {
	
	// Check to see if the joystick is currently
	// outside of the central deadzone, and that it
	// has returned to the centre before re-reading
	if x > UPPER_LIMIT || x < LOWER_LIMIT || y > UPPER_LIMIT || y < LOWER_LIMIT {
		if isJoystickCentred {
			// We're good to use the reading
			isJoystickCentred = false
			return true
		} else {
			// Ignore already moved joystick
			return false
		}
	}

	// Joystick is centred
	isJoystickCentred = true
	return false
}

func getDirection(x uint16, y uint16) uint {

	// Get player direction from the analog input
	// Centre = 32767, 32767; range 2048-65000
	ydead := y > LOWER_LIMIT && y < UPPER_LIMIT
	xdead := x > LOWER_LIMIT && x < UPPER_LIMIT

	if ydead && !xdead {
		if x < LOWER_LIMIT {
			return 3
		}

		if x > UPPER_LIMIT {
			return 1
		}
	}

	if xdead && !ydead {
		if y < LOWER_LIMIT {
			return 2
		}

		if y > UPPER_LIMIT {
			return 0
		}
	}

	if !xdead && !ydead {
		if x < LOWER_LIMIT {
			return 3
		}

		if x > UPPER_LIMIT {
			return 1
		}

		if y < LOWER_LIMIT {
			return 2
		}

		if y > UPPER_LIMIT {
			return 0
		}
	}

	return 99
}

func clearPins() {

	// Turn off the sense pins no matter what
	PIN_GREEN.Low()
	PIN_RED.Low()
}

/*
 *  Environment Functions
 */
func drawWorld() {

	// Draw the world on the 8x8 LED matrix
	// and blink the player's location
	matrix.Clear()
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			matrix.Plot(uint(i), uint(j), visited[i][j])
		}
	}

	// Flash the player's location
	matrix.Plot(player_x, player_y, isPlayerPixelOn)
	matrix.Draw()

	if time.Since(lastPlayerPixelFlash).Milliseconds() > 200 {
		isPlayerPixelOn = !isPlayerPixelOn
		lastPlayerPixelFlash = time.Now()
	}
}

func checkSenses() {

	// Present the environment to the player
	// Set the smell and draft LEDs
	// Draft = pit, Stench = Wumpus
	PIN_GREEN.Set(stink_layer[player_x][player_y])
	PIN_RED.Set(draught_layer[player_x][player_y])

	// Play a sound to signal a nearby bat
	if sound_layer[player_x][player_y] {
		tone(600, 50, 50)
		tone(500, 50, 50)
		tone(400, 50, 50)
	}
}

func checkHazards() bool {

	// Check to see if player has run into a bat, a pit or the Wumpus
	// If the player steps on a fatal square, 'check_hazards()'
	// returns true, otherwise false
	if hazards[player_x][player_y] == 'b' {
		// Player encountered a bat: play the animation...
		grabbedByBat()

		// ...then drop the player at random
		var x uint
		var y uint

		for true {
			x = irandom(0, 8)
			y = irandom(0, 8)
			if hazards[x][y] == '#' {
				break
			}
		}

		player_x = x
		player_y = y
	} else if hazards[player_x][player_y] == 'p' {
		// Player fell down a pit -- death!
		plungedIntoPit()
		gameLost()
		return true
	} else if hazards[player_x][player_y] == 'w' {
		// Player ran into the Wumpus!
		wumpusWinAnimation()
		gameLost()
		return true
	}

	return false
}

/*
 *  Player events
 */
func grabbedByBat() {

	// Show the bat flapping its wings
	// Play the animation sequence
	seq := graphics.CARRY_1[:]
	seq = append(seq, graphics.CARRY_2[:]...)
	seq = append(seq, graphics.CARRY_3[:]...)
	seq = append(seq, graphics.CARRY_4[:]...)
	seq = append(seq, graphics.CARRY_5[:]...)
	seq = append(seq, graphics.CARRY_6[:]...)
	seq = append(seq, graphics.CARRY_7[:]...)
	seq = append(seq, graphics.CARRY_8[:]...)
	seq = append(seq, graphics.CARRY_9[:]...)
	matrix.AnimateSequence(seq, 9, 100)
}

func plungedIntoPit() {

	// Show the player falling

}

/*
 *  Wumpus Attack Animations
 */
func fireArrowAnimation() {

	// Attempt to kill the Wumpus
	// Show arrow firing animation
	time.Sleep(time.Millisecond * 500)
    matrix.DrawSprite(graphics.BOW_1[:])
    tone(100, 100, 100)
    matrix.DrawSprite(graphics.BOW_2[:])
    tone(200, 100, 100)
    matrix.DrawSprite(graphics.BOW_3[:])
    tone(300, 100, 1000)
    matrix.DrawSprite(graphics.BOW_2[:])

    for i := 0 ; i < 50 ; i++ {
        tone(irandom(200, 1500), 1, 1)
    }

    matrix.DrawSprite(graphics.BOW_1[:])

    for i := 0 ; i < 25 ; i++ {
        tone(irandom(200, 1500), 1, 1)
    }

    matrix.DrawSprite(graphics.BOW_4[:])
    time.Sleep(time.Millisecond * 50)
    matrix.DrawSprite(graphics.BOW_5[:])
    time.Sleep(time.Millisecond * 100)
}

func deadWumpusAnimation() {

	// The player successfully kills the Wumpus!
	time.Sleep(time.Millisecond * 500)
    matrix.DrawSprite(graphics.WUMPUS_1[:])
    time.Sleep(time.Millisecond * 500)
    matrix.DrawSprite(graphics.WUMPUS_3[:])
    tone(900, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_4[:])
    tone(850, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_5[:])
    tone(800, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_6[:])
    tone(750, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_7[:])
    tone(700, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_8[:])
    tone(650, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_9[:])
    tone(600, 100, 100)
    matrix.DrawSprite(graphics.WUMPUS_10[:])
    tone(550, 100, 100)
    matrix.Clear()
    time.Sleep(time.Millisecond * 1000)

    // Success!
    gameWon()
}

func arrowMissAnimation() {

	// If the player misses the Wumpus

	// Show the arrow flying past...
    matrix.Clear()
    time.Sleep(time.Millisecond * 1000)

    for i := 0 ; i < 7 ; i += 2 {
        if i > 0 {
			matrix.Plot(uint(i - 2), 4, false)
		}
        matrix.Plot(uint(i), 4, true)
        matrix.Draw()
        tone(80, 100, 500)
    }

    // Clear the last arrow point
    matrix.Clear()
    matrix.Draw()

    // ...and then the Wumpus gets the player
    wumpusWinAnimation()
    gameLost()
}

func wumpusWinAnimation() {

	// Player gets attacked from the vicious Wumpus!
	// Complete with nightmare-inducing sound
	seq := graphics.WUMPUS_2[:]
	seq = append(seq, graphics.WUMPUS_1[:]...)
	for i := 0; i < 3; i++ {
		matrix.AnimateSequence(seq, 2, 250)
	}

	// Play the scream
	for i := 2000; i > 800; i -= 2 {
		tone(uint(i), 10, 1)
	}

	for i := 0; i < 5; i++ {
		matrix.AnimateSequence(seq, 2, 250)
	}
}

/*
 *  Game Outcomes
 */
func gameWon() {

	// Give the player a trophy
	clearPins()
	gameOver(textWin)
}

func gameLost() {

	// Give the player a funeral
	clearPins()
	gameOver(textLose)
}

func gameOver(text string) {

	// Show final message and
	// clear the screen for the next game
	for i := 0; i < 3; i++ {
		matrix.Print(text)
	}
	isInPlay = false
	matrix.Clear()
	matrix.Draw()
}

/*
 *  The Game's Introduction
 */
func playIntro() {

	// Callback to the theme played in the
	// version by Gregory Yob in 1975.
	// Also show the player entering the cave.
	matrix.DrawSprite(graphics.BEGIN_1[:]);
    tone(147, 200, 100);    //D3
    matrix.DrawSprite(graphics.BEGIN_2[:]);
    tone(165, 200, 100);    //E3
    matrix.DrawSprite(graphics.BEGIN_3[:]);
    tone(175, 200, 100);    //F3
    matrix.DrawSprite(graphics.BEGIN_4[:]);
    tone(196, 200, 100);    //G3
    matrix.DrawSprite(graphics.BEGIN_5[:]);
    tone(220, 200, 100);    //A4
    matrix.DrawSprite(graphics.BEGIN_6[:]);
    tone(175, 200, 100);    //F3
    matrix.DrawSprite(graphics.BEGIN_7[:]);
    tone(220, 400, 100);    //A4
    matrix.DrawSprite(graphics.BEGIN_4[:]);
    tone(208, 200, 100);    //G#3
    tone(175, 200, 100);    //E#3
    tone(208, 400, 100);    //G#3
    tone(196, 200, 100);    //G3
    tone(165, 200, 100);    //E3
    tone(196, 400, 100);    //G3
    tone(147, 200, 100);    //D3
    tone(165, 200, 100);    //E3
    tone(175, 200, 100);    //F3
    tone(196, 200, 100);    //G3
    tone(220, 200, 100);    //A3
    tone(175, 200, 100);    //F3
    tone(220, 200, 100);    //A3
    tone(294, 200, 100);    //D4
    tone(262, 200, 100);    //C4
    tone(220, 200, 100);    //A3
    tone(175, 200, 100);    //F3
    tone(220, 200, 100);    //A3
    tone(262, 400, 100);    //C4
}

/*
 *  Misc Functions
 */
func irandom(start uint, max uint) uint {

	return uint(prand.Uint32()%uint32(max) + uint32(start))
	/*
	b := make([]byte, 10)
	_, err := rand.Read(b)
	if err != nil {
		// Just return a pseudo RN
		return uint(prand.Uint32()%uint32(max) + uint32(start))
	}
	c := b[b[0]]

	return uint(c%max + start)
	*/
}

func tone(frequency uint, duration int, post uint32) {

	//time.Sleep(time.Duration(post) * time.Millisecond)
	//return
	
	// Get the cycle period in microseconds
	// NOTE Input is in Hz
	var period float32 = 1000000.0 / float32(frequency)
	period /= 2

	// Get the microsecond timer now
	start := time.Now()

	// Loop until duration (ms) in nanoseconds has elapsed
	for time.Since(start).Microseconds() < int64(duration*1000) {
		PIN_SPEAKER.High()
		time.Sleep(time.Duration(period) * time.Microsecond)
		PIN_SPEAKER.Low()
		time.Sleep(time.Duration(period) * time.Microsecond)
	}

	// Apply a post-tone delay
	time.Sleep(time.Duration(post) * time.Millisecond)
}

func failLoop() {

	// Signal hardware failure on the Pico LED
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for {
		led.Low()
		time.Sleep(time.Millisecond * 100)
		led.High()
		time.Sleep(time.Millisecond * 100)
	}
}
