package smtp

type SMTPConfig struct {
	Enabled bool
	Port    int
}

var DefaultConfig = SMTPConfig{
	Enabled: true,
	Port:    2525,
}
