# Benchmarks

Macrobenchmarks of the driver's journeys, measured on a build the way a driver gets it: minified, not debuggable.
The same journeys, in `Journeys.kt`, generate the app's baseline profile and startup profile.

| Benchmark | What it measures |
|---|---|
| `StartupBenchmark.signedIn` | Cold start to the bus screen (every shift) |
| `JourneyBenchmark.chooseBusRouteAndRun` | Bus, route, run: first opens, scrolling, search, the direction switch, Back and forward |
| `JourneyBenchmark.startTripAndUseTracking` | Starting a trip, the street map, 10 s of Tracking, the trip panel, the End trip dialog, ending the trip |
| `JourneyBenchmark.resumeAfterTheAppWasKilled` | Reopening after the app was killed mid-trip, through Resume to Tracking, then ending the trip |

Each benchmark runs twice: with no compiled code (`None`, a fresh install before Android compiles anything) and
compiled from the app's profiles (`Partial`, what a driver gets from the Play Store). Each journey starts from a fresh
app process, because the first time a screen opens is the slow case. The first launch and the first sign-in are not
benchmarked: clearing the app's data from inside a benchmark also stops the benchmark, because Android counts its
process as using the app. The profile generator still covers sign-in, since a fresh install starts signed out.

## What you need

- **An ARM device or emulator.**
  - The release build ships only ARM native libraries (MapLibre), so x86 emulators cannot run it.
  - An Apple Silicon emulator works. Use a Google APIs image, because the app needs Play services for location.
  - Boot it with the host GPU (`-gpu host`). Software rendering times the CPU instead.
- **The local benchmark server**, with `fixture/bench-gtfs.zip`:
  - 118 number-only routes;
  - route 7 runs every 30 minutes in both directions through its whole service day (03:00 to 27:00, matching the server's 03:00 cutoff), so there is always a Now run;
  - every stop lies inside the street map area below.
- **Optionally, a street map** covering lon −122.38 to −122.28, lat 47.57 to 47.64. Without one, Tracking draws on a plain background, which skips its heaviest work. One way to get it:
  `pmtiles extract https://build.protomaps.com/<date>.pmtiles seattle.pmtiles --bbox=-122.38,47.57,-122.28,47.64 --maxzoom=15`.

## Run them

1. Start Postgres and give the benchmarks their own database, so your dev data is untouched:

   ```sh
   docker compose up -d db
   docker compose exec db createdb -U postgres vehicle_positions_bench
   ```

2. Start the server from the repo root:

   ```sh
   export JWT_SECRET=$(openssl rand -hex 32) ADMIN_BOOTSTRAP_EMAIL=admin@bench.test ADMIN_BOOTSTRAP_PASSWORD=bench-admin-1
   DATABASE_URL="postgres://postgres:postgres@localhost:5432/vehicle_positions_bench?sslmode=disable" \
   GTFS_STATIC_URL=android/benchmark/fixture/bench-gtfs.zip \
   MAP_PMTILES_URL=http://10.0.2.2:8000/seattle.pmtiles \
   go run .
   ```

   If you use the street map, serve its folder: `python3 -m http.server 8000 --bind 127.0.0.1`.

3. Seed the driver and buses, once per database:

   ```sh
   ADMIN_EMAIL=$ADMIN_BOOTSTRAP_EMAIL ADMIN_PASSWORD=$ADMIN_BOOTSTRAP_PASSWORD android/benchmark/fixture/seed.sh
   ```

4. Feed GPS while the benchmarks run, so Tracking has live adherence: `ANDROID_SERIAL=emulator-5554 android/benchmark/fixture/gps.sh`.

5. Run the benchmarks from `android/`:

   ```sh
   ./gradlew :benchmark:connectedBenchmarkReleaseAndroidTest
   ```

   To run a single benchmark, add `-Pandroid.testInstrumentationRunnerArguments.tests_regex=.*chooseBusRouteAndRun.*`.
   A `class=...#chooseBusRouteAndRun` filter matches nothing, because each test's name ends in its compilation mode.

   - **A different server:** pass `-Pandroid.testInstrumentationRunnerArguments.benchServer=http://<host>:8080`, and the same for `benchEmail` and `benchPassword`.
   - **A phone over USB:** start the server with `MAP_PMTILES_URL=http://127.0.0.1:8000/seattle.pmtiles`, run
     `adb reverse tcp:8080 tcp:8080` and `adb reverse tcp:8000 tcp:8000`, and pass `benchServer=http://127.0.0.1:8080`
     with `ANDROID_SERIAL` set to the phone. Skip `gps.sh`, which only drives an emulator: the phone reports where it
     really is, so Tracking shows the bus off route, and the journeys still pass.

Results go to `benchmark/build/outputs/connected_android_test_additional_output/`, as JSON and Perfetto traces you can open in Android Studio or https://ui.perfetto.dev.

## Regenerate the profiles

After a change to the screens the journeys walk through, regenerate them with the server, map and GPS from the steps
above running:

```sh
./gradlew :app:generateReleaseBaselineProfile -Pandroid.testInstrumentationRunnerArguments.benchServer=http://10.0.2.2:8080
```

It writes `app/src/release/generated/baselineProfiles/baseline-prof.txt` and `startup-prof.txt`; commit both. The
`Partial` benchmarks fail if a build has no profile, so a missing one cannot go unnoticed.

## Reading the numbers

- **`frameDurationCpuMs`** is the time the app's code took for a frame.
- **`frameOverrunMs`** is how late the frame was against its deadline (Android 12 and later); a positive value is a dropped frame.
- Compare P50, P90, P95 and P99.
- **Startup:** `timeToInitialDisplayMs` is the first frame; `timeToFullDisplayMs` is when the screen reports it has its content.
- **Judge a change on a phone.** On an emulator the same build's frame times swing up to five times between
  iterations, mostly waiting on the Mac's GPU, which hides what a profile or a small fix changes. An emulator is still
  fine for checking that the journeys run, and its traces show what the code does.

## Finding where the time goes

Every iteration also saves a Perfetto trace next to the JSON. To see each composable by name in it (the app ships
`runtime-tracing`, which does nothing until this is on), add
`-Pandroid.testInstrumentationRunnerArguments.androidx.benchmark.fullTracing.enable=true`. Composition tracing has a
cost of its own, so compare timings from runs without it.

## Leftover data

The benchmarks sign in once and reuse the session. If the app on the device is signed in to a different server, clear
it from your computer first: `adb shell pm clear org.onebusaway.vehicletracker`.

Each journey ends the trip it starts. A run stopped in the middle of a trip leaves it open on the server, and Gradle
uninstalls the app afterwards, so the app cannot end it: the next start is refused with "You already have a trip
running". End it as the driver:

```sh
trip=$(docker compose exec -T db psql -U postgres -d vehicle_positions_bench -Atc "SELECT id FROM trips WHERE status = 'active'")
token=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"driver@bench.test","password":"bench-driver-1"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
curl -X POST http://127.0.0.1:8080/api/v1/trips/end -H "Authorization: Bearer $token" \
  -H 'Content-Type: application/json' -d "{\"trip_id\":$trip}"
```
