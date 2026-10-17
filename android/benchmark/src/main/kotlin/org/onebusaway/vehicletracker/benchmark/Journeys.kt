package org.onebusaway.vehicletracker.benchmark

import android.os.SystemClock
import androidx.benchmark.macro.MacrobenchmarkScope
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.BySelector
import androidx.test.uiautomator.Direction
import androidx.test.uiautomator.UiObject2
import androidx.test.uiautomator.Until

// The driver's journeys through the app, shared by the benchmarks and the baseline profile generator so both measure
// and warm up the same code. They run against the local benchmark server and its seeded driver (README.md), and find
// screens by the words a driver sees.

const val PACKAGE = "org.onebusaway.vehicletracker"

/** Where the benchmark server is; each value can be overridden with an instrumentation argument of that name. */
object BenchServer {
    private val args get() = InstrumentationRegistry.getArguments()
    val url: String get() = args.getString("benchServer") ?: "http://10.0.2.2:8080"
    val email: String get() = args.getString("benchEmail") ?: "driver@bench.test"
    val password: String get() = args.getString("benchPassword") ?: "bench-driver-1"
}

private const val TIMEOUT_MS = 20_000L

private val signInScreen = By.text("Use your driver account for this agency.")
private val busScreen = By.text("Choose your bus")
private val routeScreen = By.text("Choose a route")
private val runScreen = By.text("Choose a run")
private val endTripButton = By.text("End trip")

private fun MacrobenchmarkScope.waitFor(selector: BySelector, timeoutMs: Long = TIMEOUT_MS): UiObject2 =
    device.wait(Until.findObject(selector), timeoutMs) ?: error("Timed out waiting for $selector")

private fun MacrobenchmarkScope.has(selector: BySelector, timeoutMs: Long = 1_000): Boolean =
    device.wait(Until.hasObject(selector), timeoutMs) == true

/**
 * Waits for [screen] and for [previous] to leave. Taps are ignored until a screen change has finished (the app's
 * guard against double taps), and the previous screen is drawn until then.
 */
private fun MacrobenchmarkScope.arrive(screen: BySelector, previous: BySelector) {
    waitFor(screen)
    device.wait(Until.gone(previous), TIMEOUT_MS)
}

/**
 * Taps [target] until [next] appears. A tap made in the moment a screen change is finishing can still be ignored by
 * the app's double-tap guard, so a lost tap is retried; once one lands, the guard ignores the rest.
 */
private fun MacrobenchmarkScope.tapUntil(target: BySelector, next: BySelector, previous: BySelector) {
    repeat(5) {
        waitFor(target).click()
        if (device.wait(Until.hasObject(next), 3_000) == true) {
            arrive(next, previous)
            return
        }
    }
    error("Tapping $target never opened $next")
}

private fun MacrobenchmarkScope.keyboardShown(): Boolean =
    device.executeShellCommand("dumpsys input_method").contains("mInputShown=true")

/** Back closes the keyboard when it is up, and would leave the screen when it is not. */
private fun MacrobenchmarkScope.hideKeyboard() {
    if (keyboardShown()) device.pressBack()
}

/** Everything a trip start asks for, granted up front so Android's own permission screens stay out of the numbers. */
fun MacrobenchmarkScope.grantTripPermissions() {
    listOf("ACCESS_FINE_LOCATION", "ACCESS_COARSE_LOCATION", "ACCESS_BACKGROUND_LOCATION", "POST_NOTIFICATIONS").forEach {
        device.executeShellCommand("pm grant $PACKAGE android.permission.$it")
    }
    device.executeShellCommand("dumpsys deviceidle whitelist +$PACKAGE")
}

fun MacrobenchmarkScope.waitForBuses() {
    waitFor(busScreen)
}

/** Fills the first sign-in (server, email, password) and lands on the bus screen. */
fun MacrobenchmarkScope.signIn() {
    val fields = device.wait(Until.findObjects(By.clazz("android.widget.EditText")), TIMEOUT_MS)
        ?.sortedBy { it.visibleBounds.top }
        ?: error("No sign-in fields")
    check(fields.size == 3) { "Expected the first sign-in's three fields, found ${fields.size}" }
    fields[0].text = BenchServer.url
    fields[1].text = BenchServer.email
    fields[2].text = BenchServer.password
    hideKeyboard()
    // The screen's title says Sign in too; the button's label is the lower of the two.
    device.findObjects(By.text("Sign in")).maxBy { it.visibleBounds.top }.click()
    waitFor(busScreen)
}

/** Starts the app in a fresh process, signed in on the bus screen with no trip running. */
fun MacrobenchmarkScope.openFreshOnBuses() {
    killProcess()
    startActivityAndWait()
    if (has(signInScreen, 3_000)) signIn()
    endTripIfRunning()
    waitFor(busScreen)
}

fun MacrobenchmarkScope.openRoutes() {
    tapUntil(By.textContains("Bus 4317"), next = routeScreen, previous = busScreen)
}

fun MacrobenchmarkScope.openRuns() {
    tapUntil(By.desc("Route 7"), next = runScreen, previous = routeScreen)
}

/** Flings the screen's main list down then up, [times] each way. */
fun MacrobenchmarkScope.flingList(times: Int) {
    val list = device.findObjects(By.scrollable(true)).maxByOrNull { it.visibleBounds.height() }
        ?: error("No scrollable list")
    list.setGestureMargin(device.displayWidth / 5)
    repeat(times) { list.fling(Direction.DOWN); device.waitForIdle() }
    repeat(times) { list.fling(Direction.UP); device.waitForIdle() }
}

fun MacrobenchmarkScope.searchRoutes() {
    val field = waitFor(By.clazz("android.widget.EditText"))
    field.text = "7"
    SystemClock.sleep(1_000)
    field.text = ""
    SystemClock.sleep(1_000)
    hideKeyboard()
}

fun MacrobenchmarkScope.switchDirection() {
    waitFor(By.text("To Mount Baker")).click()
    SystemClock.sleep(1_000)
    waitFor(By.textStartsWith("To Downtown Seattle")).click()
    SystemClock.sleep(1_000)
}

fun MacrobenchmarkScope.backAndForward() {
    device.pressBack()
    arrive(routeScreen, previous = runScreen)
    device.pressBack()
    arrive(busScreen, previous = routeScreen)
    openRoutes()
    openRuns()
}

/**
 * Starts the run tagged Now (or Next) and waits for Tracking. On a fresh device Google's one-time Location
 * Accuracy prompt can appear, and if a permission was revoked the app's own checklist does; both are answered.
 */
fun MacrobenchmarkScope.startTripOnNowRun() {
    val run = device.findObject(By.text("Now")) ?: waitFor(By.text("Next"))
    run.click()
    val deadline = SystemClock.uptimeMillis() + 60_000
    while (SystemClock.uptimeMillis() < deadline && !has(endTripButton)) {
        device.findObject(By.text("Turn on"))?.click()
        device.findObject(By.text("Continue"))?.click()
    }
    waitFor(endTripButton)
}

/** The first trip on a device downloads the street map; the note above the trip bar goes when it is ready. */
fun MacrobenchmarkScope.waitForStreetMap() {
    device.wait(Until.gone(By.textStartsWith("Downloading map")), 60_000)
}

fun MacrobenchmarkScope.watchTracking(seconds: Long) {
    SystemClock.sleep(seconds * 1_000)
}

fun MacrobenchmarkScope.openAndClosePanel(times: Int) {
    repeat(times) {
        val bar = waitFor(endTripButton).visibleBounds
        val x = device.displayWidth / 3
        device.swipe(x, bar.centerY(), x, bar.centerY() - device.displayHeight / 2, 25)
        SystemClock.sleep(1_500)
        device.pressBack()
        SystemClock.sleep(1_500)
    }
}

fun MacrobenchmarkScope.openAndCancelEndTrip(times: Int) {
    repeat(times) {
        waitFor(endTripButton).click()
        waitFor(By.text("End this trip?"))
        waitFor(By.text("Keep going")).click()
        device.wait(Until.gone(By.text("End this trip?")), TIMEOUT_MS)
    }
}

/** Ends the running trip through the dialog: its End trip button is the lower of the two on screen. */
fun MacrobenchmarkScope.endTrip() {
    waitFor(endTripButton).click()
    waitFor(By.text("End this trip?"))
    device.findObjects(endTripButton).maxBy { it.visibleBounds.top }.click()
    waitFor(busScreen, 30_000)
}

fun MacrobenchmarkScope.resumeTrip() {
    waitFor(By.text("Resume your trip?"))
    waitFor(By.text("Resume trip")).click()
    waitFor(endTripButton)
}

/** Leaves no trip open on the server, so the next start is not refused. */
fun MacrobenchmarkScope.endTripIfRunning() {
    if (has(By.text("Resume your trip?"))) resumeTrip()
    if (has(endTripButton)) endTrip()
}
