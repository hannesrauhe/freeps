# Freeps

Freeps is a small home automation tool built around the Fritzbox. It exposes a very simple REST API and gives users the ability to model multiple actions in templates.

The project was started because the Apps available to control the FritzBox Smart Devices are too slow and lack integration features in other systems. While the FritzBox itself also offers a REST API, the authentication mechanism is not very well supported by other system. A Raspberry Pi can easily run freeps and serve as a bridge to the FritzBox.

## Install

You can create a user, install a standard systemd service and put the `freepsd` binary in place by running:

```make install```

## References

If you are looking for a Go library to interact with your FritzBox, check out [freepslib](https://github.com/hannesrauhe/freepslib).