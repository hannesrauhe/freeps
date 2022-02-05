build:
	go build -o freepsd/freepsd freepsd/freepsd.go

install: freepsd/freepsd
	mv freepsd/freepsd /usr/bin/freepsd
	adduser freeps --no-create-home --system --ingroup video
	cp systemd/freepsd.service /etc/systemd/system/freepsd.service
	systemctl daemon-reload
	systemctl restart freepsd
