var adv = true;
var state = true;
var zero = Puck.mag();
console.log(zero);

function blinkRed(time) {
  digitalPulse(LED1, 1, time);
}
function blinkGreen(time) {
  digitalPulse(LED2, 1, time);
}
function blinkBlue(time) {
  digitalPulse(LED3, 1, time);
}
function blinkYellow(time) {
  digitalPulse(LED1, 1, time);
  digitalPulse(LED2, 1, time);
}

function batteryLED() {
  blvl = Puck.getBatteryPercentage();
  if (blvl >= 75) {                // If battery is above 75%, blink green
    blinkGreen(100);
  }
  if (blvl < 75 && blvl > 25) {   // If battery is lower than 75%, blink yellow
    blinkYellow(100);
  }
  if (blvl < 25) {                 // If battery is lower than 25%, blink red!
    blinkRed(100);
  }
}

function advertise() {
  if (adv === true) {
    intstate = 0;
    if (state === true) {
      setTimeout(function () {
        blinkGreen(200);
      }, 400);
      intstate = 1;
    } else {
      setTimeout(function () {
        blinkRed(200);
      }, 400);
    }

  var p = Puck.mag();
  p.x -= zero.x;
  p.y -= zero.y;
  p.z -= zero.z;

  // divide by 200 to get a value between below 0.5 in experiments
  var magVal = Math.sqrt(p.x*p.x + p.y*p.y + p.z*p.z)/200;

  // console.log(magVal);
  NRF.setAdvertising({
      0x1809: [Math.round(E.getTemperature())],
      0x180F: [Math.round(Puck.getBatteryPercentage())],
      0x183A: [Math.round(magVal)],
      0x183B: [intstate]
    }, { name: "Puck.js 54ee", connectable: false, scannable: false, showName: true, discoverable: true, interval: 500 });
  }
}

function disableAdvertising() {
  // Disable Advertising, blink red LED to reflect this.
  setTimeout(function () {
    blinkRed(500);
  }, 400);
  NRF.setAdvertising({}); //Cancel all advertising.
}

function toggleState() {
  zero = Puck.mag();
  state = !state;
  // advertise right away
  advertise();
}

function toggleAdvertise() {
  adv = !adv;
  if (adv === true) {
    advertise();
  }
}

setInterval(advertise, 2 * 60 * 1000);

setWatch(function () {
  setTimeout(function () {
    toggleState();
  }, 500);
}, D0, { repeat: true, edge: 'rising', debounce: 49.99923706054 });
