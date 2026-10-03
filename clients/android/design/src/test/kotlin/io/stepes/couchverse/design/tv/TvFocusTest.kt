package io.stepes.couchverse.design.tv

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import com.github.takahirom.roborazzi.RobolectricDeviceQualifiers
import io.stepes.couchverse.testing.Device
import io.stepes.couchverse.testing.Fixture
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@Config(qualifiers = RobolectricDeviceQualifiers.Television1080p)
@RunWith(RobolectricTestRunner::class)
class TvFocusTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun `back on a screen focuses what had focus when it was left`() {
        var screen by mutableStateOf("shelf")
        compose.setContent {
            Fixture(Device.Tv) {
                // what a navigation host does with the screen it leaves
                val saved = rememberSaveableStateHolder()
                saved.SaveableStateProvider(screen) {
                    if (screen == "shelf") {
                        val focus = rememberScreenFocus()
                        LazyRow {
                            items((1..6).toList(), key = { it }) {
                                TvPosterCard(
                                    "Title $it",
                                    posterUrl = null,
                                    onClick = { screen = "title" },
                                    modifier = Modifier.screenFocus(focus, "$it", start = it == 1),
                                )
                            }
                        }
                    } else {
                        TvActionButton("Back", onClick = { screen = "shelf" }, modifier = Modifier.focusOnStart())
                    }
                }
            }
        }

        compose.onNodeWithContentDescription("Title 1").assertIsFocused()
        compose.onRoot().performKeyInput {
            pressKey(Key.DirectionRight)
            pressKey(Key.DirectionRight)
        }
        compose.onNodeWithContentDescription("Title 3").assertIsFocused()
        compose.onRoot().performKeyInput { pressKey(Key.DirectionCenter) }
        compose.onNodeWithText("Back").assertIsFocused()
        compose.onRoot().performKeyInput { pressKey(Key.DirectionCenter) }
        compose.onNodeWithContentDescription("Title 3").assertIsFocused()
    }

    @Test
    fun `the remote moves on from a text field`() {
        compose.setContent {
            Fixture(Device.Tv) {
                Column {
                    var text by remember { mutableStateOf("kes") }
                    OutlinedTextField(text, { text = it }, Modifier.testTag("field").remoteLeavesField(text.isEmpty()).focusOnStart())
                    TvActionButton("Search", onClick = {})
                }
            }
        }

        compose.onNodeWithTag("field").assertIsFocused()
        compose.onRoot().performKeyInput { pressKey(Key.DirectionDown) }
        compose.onNodeWithText("Search").assertIsFocused()
        compose.onRoot().performKeyInput { pressKey(Key.DirectionUp) }
        compose.onNodeWithTag("field").assertIsFocused()
    }
}
