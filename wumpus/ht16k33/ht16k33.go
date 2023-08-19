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
package ht16k33

import (
	"machine"
	"time"
	"wumpus/graphics"
)

// HT16K33 LED Matrix Commands
const (
	HT16K33_GENERIC_DISPLAY_ON uint8 = 0x81
	HT16K33_GENERIC_DISPLAY_OFF uint8 = 0x80
	HT16K33_GENERIC_SYSTEM_ON uint8 = 0x21
	HT16K33_GENERIC_SYSTEM_OFF uint8 = 0x20
	HT16K33_GENERIC_DISPLAY_ADDRESS uint8 = 0x00
	HT16K33_GENERIC_CMD_BRIGHTNESS uint8 = 0xE0
	HT16K33_GENERIC_CMD_BLINK uint8 = 0x81
	HT16K33_ADDRESS uint8 = 0x70
)

type HT16K33 struct {
	// Host I2C bus
	bus machine.I2C
	// Internal data: brightness level, buffer
	address uint8
	brightness uint
	buffer []byte
}

func New(bus machine.I2C) HT16K33 {
	
	return HT16K33{bus: bus, address: HT16K33_ADDRESS, brightness: 15, buffer: make([]byte, 8)}
}

func (p *HT16K33) Init() {

	p.Power(true)
	p.SetBrightness(2)
	p.Clear()
	p.Draw()
}

func (p *HT16K33) Power(isOn bool) {

	if isOn {
		p.i2cWriteByte(HT16K33_GENERIC_SYSTEM_ON)
		p.i2cWriteByte(HT16K33_GENERIC_DISPLAY_ON)
	} else {
		p.i2cWriteByte(HT16K33_GENERIC_DISPLAY_OFF)
		p.i2cWriteByte(HT16K33_GENERIC_SYSTEM_OFF)
	}
}

func (p *HT16K33) SetBrightness(brightness uint) {

	if brightness > 15 {
		brightness = 15
	}

	p.brightness = 15
	p.i2cWriteByte(HT16K33_GENERIC_CMD_BRIGHTNESS | byte(brightness&0xFF))
}

func (p *HT16K33) DrawSprite(sprite []byte) {

	// Write the sprite across the matrix
	// NOTE Assumes the sprite is 8 pixels wide
	copy(p.buffer, sprite)

	// Send the buffer to the LED matrix
	p.Draw()
}

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

func (p *HT16K33) Print(text string) {

	// Scroll the supplied text horizontally across the 8x8 matrix

	// Get the length of the text: the number of columns it encompasses
	length := 0
	for i := 0; i < len(text); i++ {
		ascii := text[i] - 32
		length += 2
		if ascii != 0 {
			length += len(graphics.CHARSET[ascii]) + 1
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
		ascii := text[i] - 32
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

			col += 1
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

		time.Sleep(75 * time.Millisecond)
	}
}

func (p *HT16K33) Clear() {

	// Clear the display buffer
	for i := 0; i < 8; i++ {
		p.buffer[i] = 0x00
	}
}

func (p *HT16K33) Draw() {

	// Set up the buffer holding the data to be
	// transmitted to the LED
	output_buffer := [17]byte{}

	// Span the 8 bytes of the graphics buffer
	// across the 16 bytes of the LED's buffer
	for i := 0; i < 8; i++ {
		a := p.buffer[i]
		output_buffer[i*2+1] = (a >> 1) + ((a << 7) & 0xFF)
	}

	// Write out the transmit buffer
	p.i2cWriteBlock(output_buffer[:])
}

func (p *HT16K33) AnimateSequence(sequence []byte, frameCount int, interstitialPeriod int) {

	count := 0
	for {
		frame := sequence[count:count + 8]
		p.DrawSprite(frame)
		time.Sleep(time.Millisecond * time.Duration(interstitialPeriod))

		count += 8
		if count >= (frameCount * 8) {
			break
		}
	}
}

func (p *HT16K33) i2cWriteByte(value byte) {

	// Convenience function to write a single byte to the matrix
	data := [1]byte{value}
	p.bus.Tx(uint16(HT16K33_ADDRESS), data[:], nil)
}

func (p *HT16K33) i2cWriteBlock(data []byte) {

	// Convenience function to write a 'count' bytes to the matrix
	p.bus.Tx(uint16(HT16K33_ADDRESS), data, nil)
}
