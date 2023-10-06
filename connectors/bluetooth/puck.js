var adv = true;
var stateGreen = true;
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

function advertise() {
  if (adv === false) {
    return;
  }
  var p = Puck.mag();
  p.x -= zero.x;
  p.y -= zero.y;
  p.z -= zero.z;

  var magVal = Math.sqrt(p.x * p.x + p.y * p.y + p.z * p.z) / 10;
  if (magVal > 127) {
    magVal = 127;
  }
  stateGreen = magVal < 10;

  intstate = 0;
  if (stateGreen === true) {
    setTimeout(function () {
      blinkGreen(200);
    }, 400);
    intstate = 1;
  } else {
    setTimeout(function () {
      blinkRed(200);
    }, 400);
  }

  console.log(magVal);
  NRF.setAdvertising({
    0x1809: [Math.round(E.getTemperature())],
    0x180F: [Math.round(Puck.getBatteryPercentage())],
    0x183A: [Math.round(magVal)],
    0x183B: [intstate]
  }, { name: "Puck.js 54ee", connectable: false, scannable: false, showName: true, discoverable: true, interval: 500 });
}

setInterval(advertise, 2 * 60 * 1000);

function buttonPressed() {
  zero = Puck.mag();
  // advertise right away
  advertise();
}

setWatch(function () {
  setTimeout(function () {
    buttonPressed();
  }, 500);
}, D0, { repeat: true, edge: 'rising', debounce: 49.99923706054 });



// unused functions:

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

function toggleAdvertise() {
  adv = !adv;
  if (adv === true) {
    advertise();
  }
}
