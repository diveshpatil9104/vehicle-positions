package org.onebusaway.vehicletracker.ui.map

import org.maplibre.android.LibraryLoader

private const val OPENGL_ES_3_0 = 0x30000

/**
 * Whether this phone can draw the map. MapLibre needs OpenGL ES 3.0. [glEsVersion] is
 * `ConfigurationInfo.reqGlEsVersion`: the major version in the high 16 bits, the minor in the low.
 */
fun supportsMap(glEsVersion: Int): Boolean = glEsVersion >= OPENGL_ES_3_0

/**
 * Loads MapLibre's native library ahead of the map, off the main thread, so opening Tracking does not stall a frame
 * for it: about 18 ms on a mid-range phone. A library that fails to load here is loaded again, and the failure
 * reported, when the map is first shown, as it would be without this.
 */
fun preloadMapEngine() {
    try {
        LibraryLoader.load()
    } catch (e: UnsatisfiedLinkError) {
        // Left for the map to report.
    }
}
