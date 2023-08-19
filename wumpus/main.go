/*
 * Hunt the Wumpus for Raspberry Pi Pico
 * Go version
 *
 * @version     1.0.0
 * @authors     smittytone
 * @copyright   2023, Tony Smith
 * @licence     MIT
 *
 */
package main

import (
	"machine"
	rnd "math/rand"
	"time"
	"wumpus/graphics"
	"wumpus/ht16k33"
)

func main() {

	// Set up the hardware or fail out
	if !setup() {
		failLoop()
	}

	// Play the game
	for {
		// Set up a new round...
		playIntro()

		// ...set up the environment...
		createWorld()
		drawWorld()
		_ = checkSenses(false)

		// ...and start play
		gameLoop()
	}

	return
}

/*
 * @brief Set up the game hardware.
 *
 * @returns `true` if the hardware was configured, otherwise `false`
 */
func setup() bool {

	// Configure the I2C bus
	i2c := machine.I2C0
	err := i2c.Configure(machine.I2CConfig{SCL: PIN_SCL, SDA: PIN_SDA})
	if err != nil {
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

	// Wait 2s to stabilise
	sleep(2000)
	return true
}

/*
 * @brief Roll a new board.
 */
func createWorld() {

	// The player starts at (0,0)
	startPoints := [8]uint{0, 0, 0, 7, 7, 7, 7, 0}
	startCorner := irandom(0, 4) << 1
	playerX = startPoints[startCorner]
	playerY = startPoints[startCorner+1]
	// Set the incoming direction
	if playerY == 0 {
		lastMoveDirection = UP
	} else {
		lastMoveDirection = DOWN
	}

	// Initialise the world arrays
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			hazards[i][j] = EMPTY
			visited[i][j] = false
			stinkLayer[i][j] = false
			draughtLayer[i][j] = false
			soundLayer[i][j] = false
		}
	}

	// Create 1-3 bats
	rollHazards(BAT, irandom(1, 4))

	// Create 1-3 pits
	rollHazards(PIT, irandom(1, 4))

	// Create one wumpus
	// NOTE It's generated last so bats and pits
	//      can't overwrite it by chance
	rollHazards(WUMPUS, 1)

	// Generate sense data for sounds and LED reactions
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			if hazards[i][j] == WUMPUS {
				if i < 7 {
					stinkLayer[i+1][j] = true
				}
				if i > 0 {
					stinkLayer[i-1][j] = true
				}
				if j < 7 {
					stinkLayer[i][j+1] = true
				}
				if j > 0 {
					stinkLayer[i][j-1] = true
				}
			} else if hazards[i][j] == PIT {
				if i < 7 {
					draughtLayer[i+1][j] = true
				}
				if i > 0 {
					draughtLayer[i-1][j] = true
				}
				if j < 7 {
					draughtLayer[i][j+1] = true
				}
				if j > 0 {
					draughtLayer[i][j-1] = true
				}
			} else if hazards[i][j] == BAT {
				if i < 7 {
					soundLayer[i+1][j] = true
				}
				if i > 0 {
					soundLayer[i-1][j] = true
				}
				if j < 7 {
					soundLayer[i][j+1] = true
				}
				if j > 0 {
					soundLayer[i][j-1] = true
				}
			}
		}
	}
}

/*
 * @brief Locate a hazard on the board.
 *
 * @param hazardType: The hazard to place.
 * @param count:      The number to place.
 */
func rollHazards(hazardType uint8, count uint) {

	var hazard_x uint = 0
	var hazard_y uint = 0
	var i uint
	for i = 0; i < count; i++ {
		for {
			// Make sure the rolled square is empty
			hazard_x = irandom(0, 8)
			hazard_y = irandom(0, 8)
			if hazards[hazard_x][hazard_y] == EMPTY && hazard_x != playerX && hazard_y != playerY {
				break
			}
		}

		// Place the hazard
		hazards[hazard_x][hazard_y] = hazardType
	}
}

/*
 * @brief The main game event loop.
 */
func gameLoop() {

	// Set run variables
	isInPlay = true
	debounceButtonFlag = false
	batSqueaked := false

	for {
		// Read joystick analog output
		x := PIN_X.Get()
		y := PIN_Y.Get()
		isDead := false

		if checkJoystick(x, y) {
			// The joystick is pointing in a direction,
			// so get the direction the player has chosen
			direction := getDirection(x, y)

			// Record the player's current location before the move
			visited[playerX][playerY] = true

			switch direction {
			case UP:
				if playerY < 7 {
					playerY += 1
					lastMoveDirection = UP
					batSqueaked = false
				}
			case DOWN:
				if playerY > 0 {
					playerY -= 1
					lastMoveDirection = DOWN
					batSqueaked = false
				}
			case LEFT:
				if playerX > 0 {
					playerX -= 1
					lastMoveDirection = LEFT
					batSqueaked = false
				}
			case RIGHT:
				if playerX < 7 {
					playerX += 1
					lastMoveDirection = RIGHT
					batSqueaked = false
				}
			}

			// Check the new location for sense
			// information and hazards
			isDead = checkHazards()
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
					switch lastMoveDirection {
					case UP:
						if playerY < 7 {
							if hazards[playerX][playerY+1] == WUMPUS {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					case DOWN:
						if playerY > 0 {
							if hazards[playerX][playerY-1] == WUMPUS {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					case RIGHT:
						if playerX < 7 {
							if hazards[playerX+1][playerY] == WUMPUS {
								deadWumpusAnimation()
							} else {
								arrowMissAnimation()
							}
							break
						}
					case LEFT:
						if playerX > 0 {
							if hazards[playerX-1][playerY] == WUMPUS {
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

		if isDead || !isInPlay {
			break
		} else {
			// Draw the world then check for smells and hazards
			drawWorld()
			batSqueaked = checkSenses(batSqueaked)

			// Pause between cycles
			sleep(50)
		}
	}
}

/*
 * @brief Parse the raw joystick reading to determine
 *        if it's been moved to an extreme.
 *
 * @param: x: Raw joystick x-axis reading.
 * @param: y: Raw joystick y-axis reading.
 *
 * @returns `true` if the reading is valid, otherwise `false`
 */
func checkJoystick(x uint16, y uint16) bool {

	if x > UPPER_LIMIT || x < LOWER_LIMIT || y > UPPER_LIMIT || y < LOWER_LIMIT {
		if isJoystickCentred {
			// We're good to use the reading
			isJoystickCentred = false
			return true
		} else {
			// Ignore an already moved joystick
			return false
		}
	}

	// Joystick is centred
	isJoystickCentred = true
	return false
}

/*
 * @brief Determine the direction of movement from raw
 *        joystick readings (already checked to be valid).
 *
 * @param: x: Raw joystick x-axis reading.
 * @param: y: Raw joystick y-axis reading.
 *
 * @returns The direction of movement.
 */
func getDirection(x uint16, y uint16) uint {

	// Get player direction from the analog input
	// Centre = 32767, 32767; range 2048-65000
	ydead := y > LOWER_LIMIT && y < UPPER_LIMIT
	xdead := x > LOWER_LIMIT && x < UPPER_LIMIT

	if ydead && !xdead {
		if x < LOWER_LIMIT {
			return RIGHT
		}

		if x > UPPER_LIMIT {
			return LEFT
		}
	}

	if xdead && !ydead {
		if y < LOWER_LIMIT {
			return DOWN
		}

		if y > UPPER_LIMIT {
			return UP
		}
	}

	if !xdead && !ydead {
		if x < LOWER_LIMIT {
			return RIGHT
		}

		if x > UPPER_LIMIT {
			return LEFT
		}

		if y < LOWER_LIMIT {
			return DOWN
		}

		if y > UPPER_LIMIT {
			return UP
		}
	}

	// Just in case...
	return NONE
}

/*
 * @brief Clear the small, draught LEDs.
 */
func clearPins() {

	PIN_GREEN.Low()
	PIN_RED.Low()
}

/*
 * @brief Render the map on the 8x8 matrix
 *        and flash the player's square.
 */
func drawWorld() {

	matrix.Clear()
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			matrix.Plot(uint(i), uint(j), visited[i][j])
		}
	}

	// Flash the player's location
	matrix.Plot(playerX, playerY, isPlayerPixelOn)
	matrix.Draw()

	if time.Since(lastPlayerPixelFlash).Milliseconds() > PLAYER_PIXEL_FLASH_PERIOD_MS {
		isPlayerPixelOn = !isPlayerPixelOn
		lastPlayerPixelFlash = time.Now()
	}
}

/*
 * @brief Set the signal LEDs based on the player's location.
 *
 * @param batSqueakedAlready: Sound already played, so don't repeat.
 *
 * @returns: Whether bat squeaked or not.
 */
func checkSenses(batSqueakedAlready bool) bool {

	PIN_GREEN.Set(stinkLayer[playerX][playerY])
	PIN_RED.Set(draughtLayer[playerX][playerY])

	// Play a sound to signal a nearby bat
	if soundLayer[playerX][playerY] && !batSqueakedAlready {
		tone(600, 50, 50)
		tone(500, 50, 50)
		tone(400, 50, 50)
		batSqueakedAlready = true
	}

	return batSqueakedAlready
}

/*
 * @brief Has the player stepped on a hazard?
 *
 * @returns `true` if the player has hit a hazard, `false` if they are safe.
 */
func checkHazards() bool {

	if hazards[playerX][playerY] == BAT {
		// Player encountered a bat: play the animation...
		grabbedByBatAnimation()

		// ...then drop the player at random
		var x uint
		var y uint
		for true {
			x = irandom(0, 8)
			y = irandom(0, 8)
			if hazards[x][y] == EMPTY {
				break
			}
		}

		playerX = x
		playerY = y
	} else if hazards[playerX][playerY] == PIT {
		// Player fell down a pit -> death
		plungedIntoPitAnimation()
		gameLost(false)
		return true
	} else if hazards[playerX][playerY] == WUMPUS {
		// Player ran into the Wumpus -> death
		wumpusWinAnimation()
		gameLost(true)
		return true
	}

	// Player is safe... for now!
	return false
}

/*
 * @brief Animate the player being grabbed and then
 *        dropped by a Giant Bat.
 */
func grabbedByBatAnimation() {

	sequence := graphics.CARRY_1[:]
	sequence = append(sequence, graphics.CARRY_2[:]...)
	sequence = append(sequence, graphics.CARRY_3[:]...)
	sequence = append(sequence, graphics.CARRY_4[:]...)
	sequence = append(sequence, graphics.CARRY_5[:]...)
	sequence = append(sequence, graphics.CARRY_6[:]...)
	sequence = append(sequence, graphics.CARRY_7[:]...)
	sequence = append(sequence, graphics.CARRY_8[:]...)
	sequence = append(sequence, graphics.CARRY_9[:]...)
	matrix.AnimateSequence(sequence, 9, 100)
}

/*
 * @brief Animate the player falling into a deep pit.
 */
func plungedIntoPitAnimation() {

	matrix.DrawSprite(&graphics.FALL_1)
	tone(3000, 100, 100)
	matrix.DrawSprite(&graphics.FALL_2)
	tone(2900, 100, 100)
	matrix.DrawSprite(&graphics.FALL_3)
	tone(2800, 100, 100)
	matrix.DrawSprite(&graphics.FALL_4)
	tone(2700, 100, 100)
	matrix.DrawSprite(&graphics.FALL_5)
	tone(2600, 100, 100)
	matrix.DrawSprite(&graphics.FALL_6)
	tone(2500, 100, 100)
	matrix.DrawSprite(&graphics.FALL_7)
	tone(2400, 100, 100)
	matrix.DrawSprite(&graphics.FALL_8)
	tone(2300, 100, 100)
	matrix.DrawSprite(&graphics.FALL_9)
	tone(2200, 100, 100)
	matrix.DrawSprite(&graphics.FALL_10)
	tone(2100, 100, 100)
	matrix.DrawSprite(&graphics.FALL_11)
	tone(2000, 100, 100)
	matrix.DrawSprite(&graphics.FALL_12)
	tone(1900, 100, 100)
	matrix.DrawSprite(&graphics.FALL_13)
	tone(1800, 100, 100)
	matrix.DrawSprite(&graphics.FALL_14)
	tone(1700, 100, 100)
	matrix.DrawSprite(&graphics.FALL_15)
	tone(1600, 100, 100)
	matrix.DrawSprite(&graphics.FALL_16)
	tone(1500, 100, 100)
	matrix.DrawSprite(&graphics.FALL_17)
	tone(1400, 100, 100)
}

/*
 * @brief Animate a firing bow.
 */
func fireArrowAnimation() {

	sleep(500)
	matrix.DrawSprite(&graphics.BOW_1)
	tone(100, 100, 100)
	matrix.DrawSprite(&graphics.BOW_2)
	tone(200, 100, 100)
	matrix.DrawSprite(&graphics.BOW_3)
	tone(300, 100, 1000)
	matrix.DrawSprite(&graphics.BOW_2)

	for i := 0; i < 50; i++ {
		tone(irandom(200, 1500), 1, 1)
	}

	matrix.DrawSprite(&graphics.BOW_1)

	for i := 0; i < 25; i++ {
		tone(irandom(200, 1500), 1, 1)
	}

	matrix.DrawSprite(&graphics.BOW_4)
	sleep(50)
	matrix.DrawSprite(&graphics.BOW_5)
	sleep(100)
}

/*
 * @brief Animate the death of the Wumpus.
 */
func deadWumpusAnimation() {

	// The player successfully kills the Wumpus!
	sleep(500)
	matrix.DrawSprite(&graphics.WUMPUS_1)
	sleep(500)
	matrix.DrawSprite(&graphics.WUMPUS_3)
	tone(900, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_4)
	tone(850, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_5)
	tone(800, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_6)
	tone(750, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_7)
	tone(700, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_8)
	tone(650, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_9)
	tone(600, 100, 100)
	matrix.DrawSprite(&graphics.WUMPUS_10)
	tone(550, 100, 100)
	matrix.Clear()
	sleep(1000)

	// Success!
	gameWon()
}

/*
 * @brief Animate the arrow's flight.
 */
func arrowMissAnimation() {

	// Show the arrow flying past...
	matrix.Clear()
	sleep(1000)

	for i := 0; i < 7; i += 2 {
		if i > 0 {
			matrix.Plot(uint(i-2), 4, false)
		}
		matrix.Plot(uint(i), 4, true)
		matrix.Draw()
		tone(80, 100, 500)
	}

	// Clear the last arrow point...
	matrix.Clear()
	matrix.Draw()

	// ...and then the Wumpus gets the player
	wumpusWinAnimation()
	gameLost(true)
}

/*
 * @brief Animate the Wumpus eating.
 */
func wumpusWinAnimation() {

	sequence := graphics.WUMPUS_2[:]
	sequence = append(sequence, graphics.WUMPUS_1[:]...)
	for i := 0; i < 3; i++ {
		matrix.AnimateSequence(sequence, 2, 250)
	}

	// Play the scream
	for i := 2000; i > 800; i -= 2 {
		tone(uint(i), 10, 1)
	}

	for i := 0; i < 5; i++ {
		matrix.AnimateSequence(sequence, 2, 250)
	}
}

/*
 * @brief The player won, so present the trophy graphic
 */
func gameWon() {

	clearPins()
	matrix.DrawSprite(&graphics.TROPHY)
	matrix.SetBrightness(irandom(1, 15))
	tone(1397, 100, 100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	tone(1397, 100, 100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	tone(1397, 100, 100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	tone(1397, 200, 100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	sleep(100)
	matrix.SetBrightness(irandom(7, 14))
	tone(1175, 200, 100)
	matrix.SetBrightness(irandom(1, 8))
	sleep(100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	tone(1319, 200, 100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	sleep(100)
	matrix.SetBrightness(irandom(7, 14))
	tone(1397, 200, 100)
	matrix.SetBrightness(irandom(1, 8))
	sleep(100)
	matrix.SetBrightness(irandom(7, 14))
	sleep(100)
	matrix.SetBrightness(irandom(1, 8))
	tone(1319, 150, 150)
	matrix.SetBrightness(irandom(7, 14))
	tone(1397, 400, 100)

	for i := 0; i < 6; i++ {
		matrix.SetBrightness(irandom(1, 8))
		sleep(125)
		matrix.SetBrightness(irandom(7, 14))
		sleep(125)
	}

	matrix.SetBrightness(12)
	sleep(1000)
	matrix.SetBrightness(2)

	// Show the success message
	gameOver(textWin)
}

/*
 * @brief Give the player a funeral.
 *
 * @param wumpusWon: `true` if the player died at the Wumpus' claws,
 *                   `false` if they fell into a pit
 */
func gameLost(wumpusWon bool) {

	clearPins()
	if wumpusWon {
		gameOver(textLose)
	} else {
		gameOver(textFell)
	}
}

/*
 * @brief Present the 'Game Over' text.
 */
func gameOver(text string) {

	// Show final message and
	// clear the screen for the next game
	isInPlay = false
	matrix.Print(text)
	matrix.Clear()
	matrix.Draw()
}

/*
 * @brief Present the the game's opening screen.
 */
func playIntro() {

	// A throwback to the theme played in the
	// version by Gregory Yob in 1975.
	// Also show the player entering the cave.
	matrix.DrawSprite(&graphics.BEGIN_1)
	tone(147, 200, 100) //D3
	matrix.DrawSprite(&graphics.BEGIN_2)
	tone(165, 200, 100) //E3
	matrix.DrawSprite(&graphics.BEGIN_3)
	tone(175, 200, 100) //F3
	matrix.DrawSprite(&graphics.BEGIN_4)
	tone(196, 200, 100) //G3
	matrix.DrawSprite(&graphics.BEGIN_5)
	tone(220, 200, 100) //A4
	matrix.DrawSprite(&graphics.BEGIN_6)
	tone(175, 200, 100) //F3
	matrix.DrawSprite(&graphics.BEGIN_7)
	tone(220, 400, 100) //A4
	matrix.DrawSprite(&graphics.BEGIN_4)
	tone(208, 200, 100) //G#3
	tone(175, 200, 100) //E#3
	tone(208, 400, 100) //G#3
	tone(196, 200, 100) //G3
	tone(165, 200, 100) //E3
	tone(196, 400, 100) //G3
	tone(147, 200, 100) //D3
	tone(165, 200, 100) //E3
	tone(175, 200, 100) //F3
	tone(196, 200, 100) //G3
	tone(220, 200, 100) //A3
	tone(175, 200, 100) //F3
	tone(220, 200, 100) //A3
	tone(294, 200, 100) //D4
	tone(262, 200, 100) //C4
	tone(220, 200, 100) //A3
	tone(175, 200, 100) //F3
	tone(220, 200, 100) //A3
	tone(262, 400, 100) //C4
}

/*
 * @brief Calculate a random number.
 *
 * @param start: The baseline value.
 * @param max:   One above the highest possible roll.
 *
 * @returns: The value.
 */
func irandom(start uint, max uint) uint {

	value, err := machine.GetRNG()
	if err != nil {
		return uint(rnd.Uint32()%uint32(max) + uint32(start))
	}

	return uint(uint(value)%max + start)
}

/*
 * @brief Play a sound on the piezo buzzer.
 *
 * @param frequency: The sound's frequency in Hz.
 * @param duration:  How long the sound plays in ms.
 * @param post:      A delay added after the sound has played.
 *
 * @returns: The value.
 */
func tone(frequency uint, duration int, post uint32) {

	// Get the cycle period in microseconds
	var period float32 = 1000000.0 / float32(frequency)
	period /= 2

	// Get the microsecond timer now
	start := time.Now()

	// Loop until duration (ms) in microseconds has elapsed
	for time.Since(start).Microseconds() < int64(duration*1000) {
		PIN_SPEAKER.High()
		time.Sleep(time.Duration(period) * time.Microsecond)
		PIN_SPEAKER.Low()
		time.Sleep(time.Duration(period) * time.Microsecond)
	}

	// Apply a post-tone delay
	if post != 0 {
		sleep(post)
	}
}

/*
 * @brief Flash the Pico led continuously to signal
 *        hardware setup failure.
 */
func failLoop() {

	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for {
		led.Set(!led.Get())
		sleep(100)
	}
}

/*
 * @brief Sleep for the specified period of milliseconds.
 *
 * @param period: The sleep period.
 */
func sleep(period uint32) {

	time.Sleep(time.Duration(period) * time.Millisecond)
}
