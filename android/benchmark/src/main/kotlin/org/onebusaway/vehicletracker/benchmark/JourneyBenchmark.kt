package org.onebusaway.vehicletracker.benchmark

import androidx.benchmark.macro.CompilationMode
import androidx.benchmark.macro.FrameTimingMetric
import androidx.benchmark.macro.MacrobenchmarkScope
import androidx.benchmark.macro.junit4.MacrobenchmarkRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.Parameterized

/**
 * Frame timing of each driver journey. Every iteration starts from a fresh process, so the first time each screen
 * opens is measured: that is when the app was slowest in the 2026-10-17 test. A journey that starts a trip ends it
 * too, so no trip is left open on the server when Gradle uninstalls the app after the run.
 */
@RunWith(Parameterized::class)
class JourneyBenchmark(private val compilation: CompilationMode) {
    @get:Rule val rule = MacrobenchmarkRule()

    private fun measure(setup: MacrobenchmarkScope.() -> Unit, journey: MacrobenchmarkScope.() -> Unit) =
        rule.measureRepeated(
            packageName = PACKAGE,
            metrics = listOf(FrameTimingMetric()),
            compilationMode = compilation,
            iterations = 5,
            startupMode = null,
            setupBlock = setup,
            measureBlock = journey,
        )

    @Test fun chooseBusRouteAndRun() = measure(
        setup = { grantTripPermissions(); openFreshOnBuses() },
    ) {
        openRoutes()
        flingList(times = 3)
        searchRoutes()
        openRuns()
        flingList(times = 3)
        switchDirection()
        backAndForward()
    }

    @Test fun startTripAndUseTracking() = measure(
        setup = { grantTripPermissions(); openFreshOnBuses(); openRoutes(); openRuns() },
    ) {
        startTripOnNowRun()
        waitForStreetMap()
        watchTracking(seconds = 10)
        openAndClosePanel(times = 2)
        openAndCancelEndTrip(times = 2)
        endTrip()
    }

    @Test fun resumeAfterTheAppWasKilled() = measure(
        setup = { grantTripPermissions(); openFreshOnBuses(); openRoutes(); openRuns(); startTripOnNowRun() },
    ) {
        killProcess()
        startActivityAndWait()
        resumeTrip()
        endTrip()
    }

    companion object {
        /** No compiled code at all (a fresh install), then compiled from the app's baseline profile. */
        @JvmStatic
        @Parameterized.Parameters(name = "{0}")
        fun compilationModes() = listOf(CompilationMode.None(), CompilationMode.Partial())
    }
}
