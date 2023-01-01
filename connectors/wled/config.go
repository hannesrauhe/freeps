package wled

import "fmt"

type WLEDConfig struct {
	X       int `json:",string"`
	Y       int `json:",string"`
	SegID   int `json:",string"`
	Address string
}

type OpConfig struct {
	Connections       map[string]WLEDConfig
	DefaultConnection string
}

var DefaultConfig = OpConfig{Connections: map[string]WLEDConfig{}, DefaultConnection: "default"}

func (c *WLEDConfig) Validate() error {
	if c.X <= 0 {
		return fmt.Errorf("X is not a valid width: %v", c.X)
	}
	if c.Y <= 0 {
		return fmt.Errorf("< is not a valid width: %v", c.Y)
	}
	if c.SegID < 0 {
		return fmt.Errorf("segid not a valid segment id: %v", c.SegID)
	}
	if c.Address == "" {
		return fmt.Errorf("need an address to contact WLED")
	}
	return nil
}
