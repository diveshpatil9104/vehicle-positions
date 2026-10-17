package org.onebusaway.vehicletracker.benchmark

import androidx.benchmark.macro.CompilationMode
import androidx.benchmark.macro.StartupMode
import androidx.benchmark.macro.StartupTimingMetric
import androidx.benchmark.macro.junit4.MacrobenchmarkRule
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * A cold start signed in, the way a driver opens the app every shift. The first launch, signed out, is not measured:
 * clearing the app's data from inside a benchmark also stops the benchmark, because Android counts its process as
 * using the app.
 */
@RunWith(AndroidJUnit4::class)
class StartupBenchmark {
    @get:Rule val rule = MacrobenchmarkRule()

    @Test fun signedIn() = rule.measureRepeated(
        packageName = PACKAGE,
        metrics = listOf(StartupTimingMetric()),
        compilationMode = CompilationMode.None(),
        iterations = 10,
        startupMode = StartupMode.COLD,
        setupBlock = { grantTripPermissions(); openFreshOnBuses() },
    ) {
        startActivityAndWait()
        waitForBuses()
    }
}
