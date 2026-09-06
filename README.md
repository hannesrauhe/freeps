# Freeps

Freeps is a small home automation tool built around the Fritzbox. It exposes a very simple REST API and gives users the ability to model multiple actions in templates.

The project was started because the Apps available to control the FritzBox Smart Devices are too slow and lack integration features in other systems. While the FritzBox itself also offers a REST API, the authentication mechanism is not very well supported by other system. A Raspberry Pi can easily run freeps and serve as a bridge to the FritzBox.

## Documentation

The [documentation](docs/README.md) is split into [user documentation](docs/user/)
(configuring freeps, calling its API, writing flows) and [internal documentation](docs/internal/)
(how the operator framework, the dynamic REST API and the flow engine work).

Start with [Getting started](docs/user/getting-started.md), and read the
[design principles](docs/design-principles.md) to understand the deliberate choices behind it —
that freeps exists to make the FritzBox's XML and challenge-auth API pleasant to use, that it runs
without internet access, and that it has **no authentication whatsoever** and is meant to run on a
trusted home network only.

## Install

You can create a user, install a standard systemd service and put the `freepsd` binary in place by running:

```make install```

## USB button support and other dependencies

Some of the connectors bring dependencies to specific system libraries and need additional permissions.
You can compile freeps without the support for these connectors by using build tags. Check the `Makefile` for available build tags.

In order to get support for the USB button you need to
install the following libraries on Debian-like systems:

```
apt install libudev libusb1-dev
```

, create a rule like this for udev (see `/etc/udev/rules.conf/`)

```
SUBSYSTEMS=="usb", ATTRS{idVendor}=="20a0", ATTRS{idProduct}=="42da", GROUP="users", MODE="0666"
```

and restart or reload udev:

```
udevadm control --reload-rules && udevadm trigger
```


## References

If you are looking for a Go library to interact with your FritzBox, check out [freepslib](https://github.com/hannesrauhe/freepslib).