package org.onebusaway.vehicletracker.benchmark

import androidx.benchmark.macro.CompilationMode
import androidx.benchmark.macro.StartupMode
import androidx.benchmark.macro.StartupTimingMetric
import androidx.benchmark.macro.junit4.MacrobenchmarkRule
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.Parameterized

/**
 * A cold start signed in, the way a driver opens the app every shift. The first launch, signed out, is not measured:
 * clearing the app's data from inside a benchmark also stops the benchmark, because Android counts its process as
 * using the app.
 */
@RunWith(Parameterized::class)
class StartupBenchmark(private val compilation: CompilationMode) {
    @get:Rule val rule = MacrobenchmarkRule()

    @Test fun signedIn() = rule.measureRepeated(
        packageName = PACKAGE,
        metrics = listOf(StartupTimingMetric()),
        compilationMode = compilation,
        iterations = 10,
        startupMode = StartupMode.COLD,
        setupBlock = { grantTripPermissions(); openFreshOnBuses() },
    ) {
        startActivityAndWait()
        waitForBuses()
    }

    companion object {
        /** No compiled code at all (a fresh install), then compiled from the app's baseline and startup profiles. */
        @JvmStatic
        @Parameterized.Parameters(name = "{0}")
        fun compilationModes() = listOf(CompilationMode.None(), CompilationMode.Partial())
    }
}
