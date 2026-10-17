package org.onebusaway.vehicletracker.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.onebusaway.vehicletracker.ui.permissions.ChecklistRow
import org.onebusaway.vehicletracker.ui.permissions.Grants
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.AskAgain
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.BackgroundAnswered
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.BatteryAnswered
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.Begin
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.Cancel
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.ContinueApproximate
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.LocationAnswered
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.LocationServicesChecked
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.NotificationsAnswered
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.SettingsClosed
import org.onebusaway.vehicletracker.ui.permissions.PermissionEvent.Started
import org.onebusaway.vehicletracker.ui.permissions.PermissionFlowController
import org.onebusaway.vehicletracker.ui.permissions.PermissionFlowState
import org.onebusaway.vehicletracker.ui.permissions.PermissionRow
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.APPROX_WARNING
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.CHECK_LOCATION_SETTINGS
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.IDLE
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.LOCATION_DENIED
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.LOCATION_SETTINGS_OFF
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.OPEN_APP_SETTINGS
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.READY
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.REQUEST_BACKGROUND
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.REQUEST_BATTERY
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.REQUEST_LOCATION
import org.onebusaway.vehicletracker.ui.permissions.PermissionStep.REQUEST_NOTIFICATIONS
import org.onebusaway.vehicletracker.ui.permissions.RowState
import org.onebusaway.vehicletracker.ui.permissions.RowState.ALLOWED
import org.onebusaway.vehicletracker.ui.permissions.RowState.APPROXIMATE
import org.onebusaway.vehicletracker.ui.permissions.RowState.NEEDED
import org.onebusaway.vehicletracker.ui.permissions.RowState.NEEDED_IN_SETTINGS
import org.onebusaway.vehicletracker.ui.permissions.RowState.NEXT
import org.onebusaway.vehicletracker.ui.permissions.RowState.NOT_REACHED
import org.onebusaway.vehicletracker.ui.permissions.RowState.OFF
import org.onebusaway.vehicletracker.ui.permissions.RowState.SKIPPED
import org.onebusaway.vehicletracker.ui.permissions.checklist
import org.onebusaway.vehicletracker.ui.permissions.next

/**
 * The permission sequence before a trip. What is asked, and in what order, is as it was before the checklist; the
 * checklist itself is the explanation, so Android's prompts follow one another, and a refusal for good goes to the
 * app's settings page instead of asking Android again.
 */
class PermissionStepsTest {
    private val nothingYet = Grants(sdk = 36, fine = false, coarse = false, background = false, notifications = false, batteryExempt = false)
    private val everything = Grants(sdk = 36, fine = true, coarse = true, background = true, notifications = true, batteryExempt = true)

    /** Runs [events] from IDLE, each answered with the grants beside it, and returns every step it passed through. */
    private fun walk(vararg events: Pair<PermissionEvent, Grants>): List<PermissionStep> {
        var state = PermissionFlowState()
        return events.map { (event, grants) ->
            state = state.next(event, grants)
            state.step
        }
    }

    private val fineAndCoarse = LocationAnswered(fine = true, coarse = true, canAskAgain = true)

    @Test fun `everything granted goes from location straight to the location services check`() {
        assertEquals(
            listOf(REQUEST_LOCATION, CHECK_LOCATION_SETTINGS, READY, IDLE),
            walk(Begin to everything, fineAndCoarse to everything, LocationServicesChecked(on = true) to everything, Started to everything),
        )
    }

    @Test fun `a first trip on Android 16 asks Android for each in turn`() {
        val fine = nothingYet.copy(fine = true, coarse = true)
        val background = fine.copy(background = true)
        val notified = background.copy(notifications = true)
        assertEquals(
            listOf(REQUEST_LOCATION, REQUEST_BACKGROUND, REQUEST_NOTIFICATIONS, REQUEST_BATTERY, CHECK_LOCATION_SETTINGS, READY),
            walk(
                Begin to nothingYet,
                fineAndCoarse to fine,
                BackgroundAnswered to background,
                NotificationsAnswered to notified,
                BatteryAnswered to notified.copy(batteryExempt = true),
                LocationServicesChecked(on = true) to everything,
            ),
        )
    }

    @Test fun `approximate only waits, and asking again goes back to Android`() {
        val coarse = nothingYet.copy(coarse = true)
        assertEquals(
            listOf(REQUEST_LOCATION, APPROX_WARNING, REQUEST_LOCATION, REQUEST_BACKGROUND),
            walk(
                Begin to nothingYet,
                LocationAnswered(fine = false, coarse = true, canAskAgain = true) to coarse,
                AskAgain to coarse,
                fineAndCoarse to coarse.copy(fine = true),
            ),
        )
    }

    @Test fun `going on with approximate location never asks for background location`() {
        val coarse = nothingYet.copy(coarse = true)
        assertEquals(
            listOf(REQUEST_LOCATION, APPROX_WARNING, REQUEST_NOTIFICATIONS),
            walk(
                Begin to nothingYet,
                LocationAnswered(fine = false, coarse = true, canAskAgain = true) to coarse,
                ContinueApproximate to coarse,
            ),
        )
    }

    @Test fun `refused background location, notifications and battery exemption do not stop the trip`() {
        val fine = everything.copy(background = false, notifications = false, batteryExempt = false)
        assertEquals(
            listOf(REQUEST_LOCATION, REQUEST_BACKGROUND, REQUEST_NOTIFICATIONS, REQUEST_BATTERY, CHECK_LOCATION_SETTINGS),
            walk(Begin to fine, fineAndCoarse to fine, BackgroundAnswered to fine, NotificationsAnswered to fine, BatteryAnswered to fine),
        )
    }

    @Test fun `location services off waits for the driver, and checks again once they are on`() {
        assertEquals(
            listOf(REQUEST_LOCATION, CHECK_LOCATION_SETTINGS, LOCATION_SETTINGS_OFF, CHECK_LOCATION_SETTINGS, READY),
            walk(
                Begin to everything,
                fineAndCoarse to everything,
                LocationServicesChecked(on = false) to everything,
                AskAgain to everything,
                LocationServicesChecked(on = true) to everything,
            ),
        )
    }

    @Test fun `refused location stops the trip, and once refused for good the app's settings are the way back`() {
        val refused = LocationAnswered(fine = false, coarse = false, canAskAgain = true)
        val refusedForGood = LocationAnswered(fine = false, coarse = false, canAskAgain = false)

        var state = PermissionFlowState().next(Begin, nothingYet).next(refused, nothingYet)
        assertEquals(LOCATION_DENIED, state.step)
        assertFalse(state.locationDeniedForGood)
        assertNull("Continue waits until location is allowed", state.continueEvent)

        state = state.next(AskAgain, nothingYet)
        assertEquals(REQUEST_LOCATION, state.step)
        state = state.next(refusedForGood, nothingYet)
        assertEquals(LOCATION_DENIED, state.step)
        assertTrue(state.locationDeniedForGood)

        // Before the checklist, asking again here asked Android, which refused at once, round and round.
        state = state.next(AskAgain, nothingYet)
        assertEquals(OPEN_APP_SETTINGS, state.step)
        state = state.next(SettingsClosed, nothingYet.copy(fine = true, coarse = true))
        assertEquals(REQUEST_LOCATION, state.step)
        assertEquals(REQUEST_BACKGROUND, state.next(fineAndCoarse, nothingYet.copy(fine = true, coarse = true)).step)
    }

    @Test fun `going back leaves the sequence wherever it is, and the next run starts afresh`() {
        for (step in PermissionStep.entries) {
            assertEquals("from $step", PermissionFlowState(), PermissionFlowState(step).next(Cancel, everything))
            assertEquals("from $step", PermissionFlowState(REQUEST_LOCATION), PermissionFlowState(step).next(Begin, everything))
        }
    }

    @Test fun `Android 8 has no background location or notification permission to ask for`() {
        val oreo = nothingYet.copy(sdk = 26, fine = true, coarse = true, batteryExempt = true)
        assertEquals(listOf(REQUEST_LOCATION, CHECK_LOCATION_SETTINGS), walk(Begin to oreo, fineAndCoarse to oreo))
    }

    @Test fun `Android 10 asks for background location but not notifications`() {
        val q = nothingYet.copy(sdk = 29, fine = true, coarse = true, batteryExempt = true)
        assertEquals(
            listOf(REQUEST_LOCATION, REQUEST_BACKGROUND, CHECK_LOCATION_SETTINGS),
            walk(Begin to q, fineAndCoarse to q, BackgroundAnswered to q.copy(background = true)),
        )
    }

    @Test fun `Android 13 and 16 ask for notifications too`() {
        for (sdk in listOf(33, 36)) {
            val granted = everything.copy(sdk = sdk, notifications = false)
            assertEquals(
                "on API $sdk",
                listOf(REQUEST_LOCATION, REQUEST_NOTIFICATIONS, CHECK_LOCATION_SETTINGS),
                walk(Begin to granted, fineAndCoarse to granted, NotificationsAnswered to everything.copy(sdk = sdk)),
            )
        }
    }

    @Test fun `an answer that does not belong to the step changes nothing`() {
        val waiting = PermissionFlowState(REQUEST_NOTIFICATIONS)
        for (event in listOf(fineAndCoarse, AskAgain, ContinueApproximate, SettingsClosed, BackgroundAnswered, BatteryAnswered, LocationServicesChecked(true), Started)) {
            assertEquals("after $event", waiting, waiting.next(event, everything))
        }
        assertEquals(PermissionFlowState(), PermissionFlowState().next(LocationServicesChecked(true), everything))
    }

    @Test fun `Continue starts the walk, or goes on with approximate location, and waits otherwise`() {
        assertEquals(Begin, PermissionFlowState(IDLE).continueEvent)
        assertEquals(ContinueApproximate, PermissionFlowState(APPROX_WARNING).continueEvent)
        for (step in PermissionStep.entries - setOf(IDLE, APPROX_WARNING)) {
            assertNull("at $step", PermissionFlowState(step).continueEvent)
        }
    }

    // --- The checklist's rows ---

    private fun PermissionFlowState.states(grants: Grants): List<RowState> = checklist(grants).map(ChecklistRow::state)

    @Test fun `before Continue, rows already granted say so and the first missing one is next`() {
        assertEquals(
            listOf(NEXT, NOT_REACHED, NOT_REACHED, NOT_REACHED, NOT_REACHED, NOT_REACHED),
            PermissionFlowState().states(nothingYet),
        )
        assertEquals(
            listOf(ALLOWED, ALLOWED, NEXT, ALLOWED, NOT_REACHED, NOT_REACHED),
            PermissionFlowState().states(everything.copy(background = false, batteryExempt = false)),
        )
        assertEquals(PermissionRow.entries, PermissionFlowState().checklist(nothingYet).map(ChecklistRow::row))
    }

    @Test fun `as the walk goes on, rows behind it are allowed or skipped`() {
        val noBackground = everything.copy(background = false, notifications = false)
        assertEquals(
            listOf(ALLOWED, ALLOWED, SKIPPED, SKIPPED, NEXT, NOT_REACHED),
            PermissionFlowState(REQUEST_BATTERY).states(noBackground.copy(batteryExempt = false)),
        )
        assertEquals(
            listOf(ALLOWED, ALLOWED, ALLOWED, ALLOWED, ALLOWED, ALLOWED),
            PermissionFlowState(READY).states(everything),
        )
    }

    @Test fun `a row that stops the walk says what the driver can do`() {
        assertEquals(NEEDED, PermissionFlowState(LOCATION_DENIED).states(nothingYet)[0])
        assertEquals(NEEDED_IN_SETTINGS, PermissionFlowState(LOCATION_DENIED, locationDeniedForGood = true).states(nothingYet)[0])
        val coarse = nothingYet.copy(coarse = true)
        assertEquals(listOf(ALLOWED, APPROXIMATE), PermissionFlowState(APPROX_WARNING).states(coarse).take(2))
        assertEquals(OFF, PermissionFlowState(LOCATION_SETTINGS_OFF).states(everything).last())
    }

    @Test fun `on Android 8, location is all the time and notifications need no asking`() {
        val oreo = nothingYet.copy(sdk = 26, coarse = true)
        assertEquals(listOf(ALLOWED, NEXT, ALLOWED, ALLOWED), PermissionFlowState().states(oreo).take(4))
    }

    // --- When the checklist is on screen ---

    private class PhoneGrants(var now: Grants)

    @Test fun `a run with everything granted starts without the checklist`() {
        val controller = PermissionFlowController { everything }

        controller.start()

        assertFalse(controller.visible)
        assertEquals(REQUEST_LOCATION, controller.flow.step)
    }

    @Test fun `a run with something missing shows the checklist and waits for Continue`() {
        val controller = PermissionFlowController { everything.copy(batteryExempt = false) }

        controller.start()

        assertTrue(controller.visible)
        assertEquals(IDLE, controller.flow.step)
        assertTrue(controller.canContinue)
        controller.onContinue()
        assertEquals(REQUEST_LOCATION, controller.flow.step)
    }

    @Test fun `a run that needed nothing shows the checklist once a step waits on the driver`() {
        val controller = PermissionFlowController { everything }
        controller.start()
        controller.dispatch(fineAndCoarse)
        assertFalse(controller.visible)

        controller.dispatch(LocationServicesChecked(on = false))

        assertTrue(controller.visible)
        assertEquals(OFF, controller.checklist.last().state)
    }

    @Test fun `the checklist goes once the run starts, or the driver goes back`() {
        val controller = PermissionFlowController { nothingYet }
        controller.start()
        controller.started()
        assertFalse(controller.visible)
        assertEquals(IDLE, controller.flow.step)

        controller.start()
        controller.onContinue()
        controller.cancel()
        assertFalse(controller.visible)
        assertEquals(IDLE, controller.flow.step)
    }

    @Test fun `rows follow what was changed in Android's settings while the app was away`() {
        val phone = PhoneGrants(nothingYet)
        val controller = PermissionFlowController { phone.now }
        controller.start()
        assertEquals(NEXT, controller.checklist.first().state)

        phone.now = nothingYet.copy(fine = true, coarse = true)
        controller.refresh()

        assertEquals(listOf(ALLOWED, ALLOWED, NEXT), controller.checklist.take(3).map(ChecklistRow::state))
    }

    @Test fun `the checklist shows only when something is missing`() {
        assertTrue(everything.hasEverything)
        assertTrue("Android 8 has no background or notification permission to miss", everything.copy(sdk = 26, background = false, notifications = false).hasEverything)
        assertFalse(everything.copy(fine = false).hasEverything)
        assertFalse(everything.copy(background = false).hasEverything)
        assertFalse(everything.copy(notifications = false).hasEverything)
        assertFalse(everything.copy(batteryExempt = false).hasEverything)
    }
}
