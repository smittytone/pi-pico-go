/*
 * Hunt the Wumpus for Raspberry Pi Pico
 * Go version
 *
 * @authors     smittytone
 * @copyright   2024, Tony Smith
 * @licence     MIT
 *
 */
package ht16k33

import (
	"machine"
	"time"
	"wumpus/graphics"
)

const (
	HT16K33_CMD_DISPLAY_ON      uint8 = 0x81
	HT16K33_CMD_DISPLAY_OFF     uint8 = 0x80
	HT16K33_CMD_SYSTEM_ON       uint8 = 0x21
	HT16K33_CMD_SYSTEM_OFF      uint8 = 0x20
	HT16K33_FRAME_STORE_ADDRESS uint8 = 0x00
	HT16K33_CMD_BRIGHTNESS      uint8 = 0xE0
	HT16K33_CMD_BLINK           uint8 = 0x81
	HT16K33_ADDRESS             uint8 = 0x70
)

type HT16K33 struct {
	// Host I2C bus
	bus machine.I2C
	// Internal data: I2C address, brightness level, frame buffer
	address    uint8
	brightness uint
	buffer     [8]byte
}

/*
 * @brief Convenience method to instantiate and initialise
 *        an HT16K33 struct.
 *
 * @param bus:     A TinyGo machine.I2C instance.
 *                 IMPORTANT This must be configured by the calling
 *                           application BEFORE calling `init()` or any
 *                           other HT16K33 method.
 * @param address: The display's 7-bit I2C address. Defaults to `0x70`
 *                 if out of range.
 */
func New(bus machine.I2C, address uint8) HT16K33 {

	if address < 8 || address > 0xF0 {
		address = HT16K33_ADDRESS
	}
	return HT16K33{
		bus:        bus,
		address:    address,
		brightness: 15,
		buffer:     [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
	}
}

/*
 * @brief Convenience method to power on the display, set a default
 *        brightness, clear the frame buffer and write the buffer to
 *        the display.
 */
func (p *HT16K33) Init() {

	p.Power(true)
	p.SetBrightness(8)
	p.Clear()
	p.Draw()
}

/*
 * @brief Turn the display on or off.
 *        The display must be turned on before it can be used
 *        (by calling `Draw()`).
 *
 * @param isOn: `true` to enable the display, `false` to turn it off.
 */
func (p *HT16K33) Power(isOn bool) {

	if isOn {
		p.i2cWriteByte(HT16K33_CMD_SYSTEM_ON)
		p.i2cWriteByte(HT16K33_CMD_DISPLAY_ON)
	} else {
		p.i2cWriteByte(HT16K33_CMD_DISPLAY_OFF)
		p.i2cWriteByte(HT16K33_CMD_SYSTEM_OFF)
	}
}

/*
 * @brief Set the display's brightness.
 *        The effect is immediate.
 *
 * @param brightness: A value between 0 (dim) and 15 (very bright).
 *                    Note that 0 does not turn off the display.
 */
func (p *HT16K33) SetBrightness(brightness uint) {

	if brightness > 15 {
		brightness = 15
	}

	p.brightness = brightness
	p.i2cWriteByte(HT16K33_CMD_BRIGHTNESS | byte(brightness&0xFF))
}

/*
 * @brief Write a graphic pattern to the frame buffer.
 *        Doesn't update the display -- call `Draw()` to do so.
 *
 * @param sprite: A graphic stored as an [8]byte array.
 */
func (p *HT16K33) DrawSprite(sprite *graphics.Sprite) {

	// Write the sprite across the matrix
	// NOTE Assumes the sprite is 8 pixels wide
	p.buffer = *sprite

	// Send the buffer to the LED matrix
	p.Draw()
}

/*
 * @brief Turn a specific pixel on the 8x8 matrix on or off.
 *        (0,0) is the bottom left corner as per the orientation
 *        in the circuit diagram here:
 *        https://github.com/smittytone/pi-pico-go
 *        Doesn't update the display -- call `Draw()` to do so.
 *
 * @param x:     The pixel's X co-ordinate.
 * @param y:     The pixel's Y co-ordinate.
 * @param isSet: `true` to light the pixel, `false` to clear it.
 */
func (p *HT16K33) Plot(x uint, y uint, isSet bool) {

	// Set or unset the specified pixel
	col := p.buffer[x]

	if isSet {
		col |= (1 << y)
	} else {
		col &= ^(1 << y)
	}

	p.buffer[x] = col
}

/*
 * @brief Scroll a text string across the display.
 *        The text should only contain valid Ascii characters.
 *
 * @param text: The string to scroll.
 */
func (p *HT16K33) Print(text string) {

	// Get the length of the text: the number of columns it encompasses
	length := 0
	for i := 0; i < len(text); i++ {
		ascii := int(text[i]) - 32
		if ascii != 0 {
			length += len(graphics.CHARSET[ascii]) + 1
		} else {
			length += 2
		}
	}

	// Make the output buffer to match the required number of columns
	src_buffer := make([]byte, length)
	for i := 0; i < length; i++ {
		src_buffer[i] = 0x00
	}

	// Write each character's glyph columns into the output buffer
	col := 0
	for i := 0; i < len(text); i++ {
		ascii := int(text[i]) - 32
		if ascii == 0 {
			// It's a space, so just add two blank columns
			col += 2
		} else {
			// Get the character glyph and write it to the buffer
			glyphWidth := len(graphics.CHARSET[ascii])

			for j := 0; j < glyphWidth; j++ {
				src_buffer[col] = graphics.CHARSET[ascii][j]
				col += 1
			}
		}
	}

	// Finally, animate the line by repeatedly sending 8 columns
	// of the output buffer to the matrix
	cursor := 0
	for {
		a := cursor
		for i := 0; i < 8; i++ {
			p.buffer[i] = src_buffer[a]
			a += 1
		}

		p.Draw()
		cursor += 1
		if cursor > length-8 {
			break
		}

		// Pause between frames
		time.Sleep(80 * time.Millisecond)
	}
}

/*
 * @brief Clear the internal frame buffer.
 *        Doesn't update the display -- call `Draw()` to do so.
 */
func (p *HT16K33) Clear() {

	// NOTE Better (no garbage collection) to populate the
	//      existing array than just create a new one?
	for i := 0; i < 8; i++ {
		p.buffer[i] = 0x00
	}
}

/*
 * @brief Write the internal frame buffer to the display.
 */
func (p *HT16K33) Draw() {

	// Set up the buffer holding the data to be transmitted
	output_buffer := [17]byte{}

	// Span the 8 bytes of the frame buffer
	// across the 16 bytes of the TX buffer
	for i := 0; i < 8; i++ {
		a := p.buffer[i]
		output_buffer[i*2+1] = (a >> 1) + ((a << 7) & 0xFF)
	}

	// Write out the transmit buffer
	p.i2cWriteBlock(output_buffer[:])
}

/*
 * @brief Display a series of 8x8 frames on the display.
 *
 * @param sequence:           A slice containing all the frames in order.
 * @param frameCount:         The number of 8x8 frames in the sequence.
 * @param interstitialPeriod: The time in ms between frames.
 */
func (p *HT16K33) AnimateSequence(sequence []byte, frameCount int, interstitialPeriod int) {

	count := 0
	for {
		frame := graphics.Sprite{}
		copy(frame[:], sequence[count:count+8])
		p.DrawSprite(&frame)
		time.Sleep(time.Millisecond * time.Duration(interstitialPeriod))

		count += 8
		if count >= (frameCount * 8) {
			break
		}
	}
}

/*
 * @brief Write a byte to I2C.
 *
 * @param value: The byte to write.
 */
func (p *HT16K33) i2cWriteByte(value byte) {

	// Convenience function to write a single byte to the matrix
	data := [1]byte{value}
	p.bus.Tx(uint16(HT16K33_ADDRESS), data[:], nil)
}

/*
 * @brief Write a series of bytes to I2C.
 *
 * @param value: A slice of the bytes to write.
 */
func (p *HT16K33) i2cWriteBlock(data []byte) {

	// Convenience function to write a 'count' bytes to the matrix
	p.bus.Tx(uint16(HT16K33_ADDRESS), data, nil)
}
