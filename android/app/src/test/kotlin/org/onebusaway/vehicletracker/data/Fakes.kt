package org.onebusaway.vehicletracker.data

import kotlinx.coroutines.flow.MutableStateFlow
import org.onebusaway.vehicletracker.data.map.MapFileSource
import org.onebusaway.vehicletracker.data.map.MapFileState
import org.onebusaway.vehicletracker.engine.TripGeometry
import org.onebusaway.vehicletracker.service.ServiceController
import java.security.GeneralSecurityException
import java.util.Base64

/**
 * Stands in for [KeystoreCryptor] so session and migration logic runs on the JVM, where CI
 * actually executes it.
 *
 * It reverses the value rather than just tagging it, so a test can assert that the plaintext
 * token never appears in the bytes on disk, and [decrypt] rejects anything it did not produce,
 * so a token written in the clear cannot read back as if it had been encrypted.
 */
class FakeCryptor : Cryptor {
    var failEncrypt = false
    var failDecrypt = false

    /**
     * Thrown from [encrypt] when [failEncrypt] is set. Set it to a `ProviderException` to stand in
     * for a device whose Keystore reports failures as an unchecked exception.
     */
    var encryptFailure: Exception = GeneralSecurityException("keystore unavailable")

    // Thrown raw, the way the Keystore itself does. Normalising here instead would make the store
    // tests pass whether or not KeystoreCryptor converts anything, which is no test at all.
    override fun encrypt(plaintext: String): String {
        if (failEncrypt) throw encryptFailure
        return PREFIX + plaintext.reversed()
    }

    override fun decrypt(ciphertext: String): String {
        if (failDecrypt) throw GeneralSecurityException("key no longer usable")
        if (!ciphertext.startsWith(PREFIX)) throw GeneralSecurityException("not ciphertext")
        return ciphertext.removePrefix(PREFIX).reversed()
    }

    private companion object {
        const val PREFIX = "enc:"
    }
}

class FakeMapFileSource : MapFileSource {
    override val state = MutableStateFlow<MapFileState>(MapFileState.None)
}

class FakeSessionStore : SessionStore {
    val state = MutableStateFlow(Session(null, null, null))
    override val session = state
    override suspend fun saveLogin(serverUrl: String, email: String, token: String, issuedAtEpochSec: Long) {
        state.value = Session(serverUrl, token, issuedAtEpochSec, email)
    }
    override suspend fun clearToken() { state.value = state.value.copy(token = null, issuedAtEpochSec = null) }
}

/** A token shaped like the server's JWT, carrying [sub]; the phone never checks its signature. */
fun jwtFor(sub: String): String {
    val encoder = Base64.getUrlEncoder().withoutPadding()
    fun part(json: String) = encoder.encodeToString(json.toByteArray())
    return part("""{"alg":"HS256","typ":"JWT"}""") + "." + part("""{"sub":"$sub","exp":4102444800}""") + ".signature"
}

fun signedInAs(sub: String) = FakeSessionStore().apply { state.value = Session("http://localhost/", jwtFor(sub), 0L) }

class FakeTripStateStore : TripStateStore {
    val tripState = MutableStateFlow<ActiveTrip?>(null)
    val routesState = MutableStateFlow<List<String>>(emptyList())
    val unconfirmedEndState = MutableStateFlow<UnconfirmedEnd?>(null)
    override val activeTrip = tripState
    override val unconfirmedEnd = unconfirmedEndState
    override val recentRoutes = routesState
    override suspend fun saveActiveTrip(trip: ActiveTrip) { tripState.value = trip }
    override suspend fun clearActiveTrip() { tripState.value = null }
    override suspend fun endActiveTripLocally(driverId: String?) {
        tripState.value?.let { unconfirmedEndState.value = UnconfirmedEnd(it.tripDbId, driverId) }
        tripState.value = null
    }
    override suspend fun clearUnconfirmedEnd() { unconfirmedEndState.value = null }
    override suspend fun addRecentRoute(routeId: String) {
        routesState.value = (listOf(routeId) + routesState.value.filter { it != routeId }).take(5)
    }
}

class FakeVehiclePrefsStore : VehiclePrefsStore {
    val favoritesState = MutableStateFlow<Set<String>>(emptySet())
    val recentsState = MutableStateFlow<List<String>>(emptyList())
    override val favorites = favoritesState
    override val recents = recentsState
    override suspend fun toggleFavorite(vehicleId: String) {
        val current = favoritesState.value
        favoritesState.value = if (vehicleId in current) current - vehicleId else current + vehicleId
    }
    override suspend fun recordUse(vehicleId: String) {
        recentsState.value = (listOf(vehicleId) + recentsState.value.filter { it != vehicleId }).take(5)
    }
}

class FakeTripGeometryStore : TripGeometryStore {
    var stored: TripGeometry? = null
    override suspend fun save(geometry: TripGeometry) { stored = geometry }
    override suspend fun load() = stored
    override suspend fun clear() { stored = null }
}

class FakeServiceController : ServiceController {
    var startCount = 0
    var stopCount = 0
    override fun startTracking() { startCount++ }
    override fun stopTracking() { stopCount++ }
}
