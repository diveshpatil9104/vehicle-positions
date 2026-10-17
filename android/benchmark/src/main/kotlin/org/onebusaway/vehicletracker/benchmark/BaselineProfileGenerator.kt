package org.onebusaway.vehicletracker.benchmark

import androidx.benchmark.macro.junit4.BaselineProfileRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Writes the app's baseline profile and startup profile from the same journeys the benchmarks measure. Run by hand
 * against the local benchmark server (README.md); the output goes to app/src/release/generated/baselineProfiles and
 * is committed.
 */
@RunWith(AndroidJUnit4::class)
class BaselineProfileGenerator {
    @get:Rule val rule = BaselineProfileRule()

    /** Opening the app signed in, every shift; the first run on a fresh install also signs in. */
    @Test fun startup() = rule.collect(packageName = PACKAGE, maxIterations = 8, includeInStartupProfile = true) {
        grantTripPermissions()
        openFreshOnBuses()
    }

    /** Everything after the bus screen: choosing the trip, Tracking, ending, and Resume after the app was killed. */
    @Test fun driverJourneys() = rule.collect(packageName = PACKAGE, maxIterations = 8) {
        grantTripPermissions()
        openFreshOnBuses()
        openRoutes()
        flingList(times = 1)
        searchRoutes()
        openRuns()
        flingList(times = 1)
        switchDirection()
        backAndForward()
        startTripOnNowRun()
        waitForStreetMap()
        watchTracking(seconds = 3)
        openAndClosePanel(times = 1)
        openAndCancelEndTrip(times = 1)
        endTrip()
        openRoutes()
        openRuns()
        startTripOnNowRun()
        killProcess()
        startActivityAndWait()
        resumeTrip()
        endTrip()
    }
}
