package io.stepes.couchverse.design.phone

import androidx.compose.material3.ButtonColors
import androidx.compose.material3.ButtonDefaults
import androidx.compose.runtime.Composable
import io.stepes.couchverse.design.theme.LocalAccent

/** A text button's label in the accent's ink, which stays readable where the accent would not. */
@Composable
fun inkButtonColors(): ButtonColors = ButtonDefaults.textButtonColors(contentColor = LocalAccent.current.ink)
