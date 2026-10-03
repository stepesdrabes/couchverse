package io.stepes.couchverse.ranks

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import io.stepes.couchverse.core.Event
import io.stepes.couchverse.core.ImageChoice
import io.stepes.couchverse.core.ImageSlot
import io.stepes.couchverse.core.ImageSlotRef
import io.stepes.couchverse.core.LoadStatus
import io.stepes.couchverse.core.PasswordForm
import io.stepes.couchverse.core.ProfileEdit
import io.stepes.couchverse.core.ProfileEditorView
import io.stepes.couchverse.core.SaveState
import io.stepes.couchverse.core.SessionUser
import io.stepes.couchverse.core.SessionView
import io.stepes.couchverse.core.Surface
import io.stepes.couchverse.design.R
import io.stepes.couchverse.design.Tokens
import io.stepes.couchverse.design.components.Artwork
import io.stepes.couchverse.design.components.Avatar
import io.stepes.couchverse.design.phone.PhoneGutter
import io.stepes.couchverse.design.phone.inkButtonColors
import io.stepes.couchverse.design.runtime.rememberSend
import io.stepes.couchverse.design.runtime.rememberSurface
import io.stepes.couchverse.design.text.problemMessage
import io.stepes.couchverse.design.theme.LocalIsTv
import io.stepes.couchverse.design.tv.TvActionButton
import io.stepes.couchverse.design.tv.TvSafe
import io.stepes.couchverse.design.tv.focusOnStart
import io.stepes.couchverse.design.tv.remoteLeavesField

class ProfileEditorActions(
    val onSave: (ProfileEdit) -> Unit,
    val onPassword: (PasswordForm) -> Unit,
    /** The picked image's content URI; absent on a TV, which edits pictures elsewhere. */
    val onPick: ((ImageSlot) -> Unit)?,
    val onRemove: (ImageSlot) -> Unit,
    val onBack: (() -> Unit)?,
)

/** What the editor shows of the account now: its pictures and current name and bio. */
class ProfileEditorState(
    val user: SessionUser?,
    val avatarUrl: String?,
    val bannerUrl: String?,
    val view: ProfileEditorView?,
)

/**
 * Editing the viewer's profile: the avatar and banner (picked with the system photo picker on
 * a phone), the name and bio, and the password, each saved on its own.
 */
@Composable
fun ProfileEditorScreen(state: ProfileEditorState, actions: ProfileEditorActions) {
    val tv = LocalIsTv.current
    val user = state.user
    var name by rememberSaveable(user?.displayName) { mutableStateOf(user?.displayName.orEmpty()) }
    var bio by rememberSaveable(user?.bio) { mutableStateOf(user?.bio.orEmpty()) }
    var current by rememberSaveable { mutableStateOf("") }
    var new by rememberSaveable { mutableStateOf("") }
    var confirm by rememberSaveable { mutableStateOf("") }
    Column(
        Modifier
            .fillMaxSize()
            .then(if (tv) Modifier.padding(horizontal = TvSafe.horizontal, vertical = TvSafe.vertical) else Modifier.statusBarsPadding().imePadding())
            .verticalScroll(rememberScrollState())
            .padding(horizontal = if (tv) 0.dp else PhoneGutter, vertical = 8.dp)
            .widthIn(max = 640.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (!tv) {
                actions.onBack?.let { back ->
                    IconButton(onClick = back) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.common_back)) }
                }
            }
            Text(stringResource(R.string.profile_heading), style = MaterialTheme.typography.headlineSmall, color = Tokens.Palette.text, modifier = Modifier.semantics { heading() })
        }

        actions.onPick?.let { pick ->
            Pictures(state, pick, actions.onRemove)
        }

        Field(name, { name = it }, R.string.profile_display_name, start = true)
        Field(bio, { bio = it.take(BIO_LIMIT) }, R.string.profile_bio, singleLine = false, placeholder = R.string.profile_bio_placeholder)
        SaveRow(state.view?.details, R.string.profile_saved, R.string.profile_save_failed) {
            Action(stringResource(R.string.common_save), primary = true, enabled = name.isNotBlank()) { actions.onSave(ProfileEdit(name.trim(), bio)) }
        }

        Text(stringResource(R.string.profile_password_heading), style = MaterialTheme.typography.titleMedium, color = Tokens.Palette.text, modifier = Modifier.semantics { heading() })
        Field(current, { current = it }, R.string.profile_current_password, password = true)
        Field(new, { new = it }, R.string.profile_new_password, password = true)
        Field(confirm, { confirm = it }, R.string.profile_confirm_password, password = true)
        val problem = when {
            new.isNotEmpty() && new.length < MIN_PASSWORD -> pluralStringResource(R.plurals.profile_password_too_short, MIN_PASSWORD, MIN_PASSWORD)
            confirm.isNotEmpty() && confirm != new -> stringResource(R.string.profile_password_mismatch)
            else -> null
        }
        problem?.let { Text(it, color = Tokens.Palette.danger, style = MaterialTheme.typography.bodySmall) }
        SaveRow(state.view?.password, R.string.profile_password_changed, R.string.profile_password_change_failed) {
            Action(stringResource(R.string.profile_password_heading), primary = false, enabled = problem == null && current.isNotEmpty() && new.isNotEmpty() && confirm == new) {
                actions.onPassword(PasswordForm(current, new))
            }
        }
    }
}

@Composable
private fun Pictures(state: ProfileEditorState, pick: (ImageSlot) -> Unit, remove: (ImageSlot) -> Unit) {
    Box(Modifier.fillMaxWidth().aspectRatio(3f).clip(RoundedCornerShape(Tokens.Radius.card))) {
        Artwork(state.bannerUrl, modifier = Modifier.fillMaxSize())
        if (state.view?.banner?.status == LoadStatus.Loading) CircularProgressIndicator(Modifier.align(Alignment.Center))
    }
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedButton(onClick = { pick(ImageSlot.Banner) }) { Text(stringResource(if (state.bannerUrl == null) R.string.profile_banner_upload else R.string.profile_banner_replace)) }
        if (state.bannerUrl != null) TextButton(onClick = { remove(ImageSlot.Banner) }, colors = inkButtonColors()) { Text(stringResource(R.string.profile_banner_remove)) }
    }
    state.view?.banner?.problem?.let { Text(problemMessage(it), color = Tokens.Palette.danger) }
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        Box {
            Avatar(state.avatarUrl, seed = state.user?.username.orEmpty(), size = 72.dp)
            if (state.view?.avatar?.status == LoadStatus.Loading) CircularProgressIndicator(Modifier.align(Alignment.Center))
        }
        OutlinedButton(onClick = { pick(ImageSlot.Avatar) }) { Text(stringResource(R.string.profile_change_picture)) }
        if (state.avatarUrl != null) TextButton(onClick = { remove(ImageSlot.Avatar) }, colors = inkButtonColors()) { Text(stringResource(R.string.profile_remove_picture)) }
    }
    state.view?.avatar?.problem?.let { Text(problemMessage(it), color = Tokens.Palette.danger) }
}

@Composable
private fun Field(
    value: String,
    onChange: (String) -> Unit,
    label: Int,
    singleLine: Boolean = true,
    password: Boolean = false,
    placeholder: Int? = null,
    start: Boolean = false,
) {
    val tv = LocalIsTv.current
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(stringResource(label)) },
        placeholder = placeholder?.let { { Text(stringResource(it)) } },
        singleLine = singleLine,
        minLines = if (singleLine) 1 else 3,
        visualTransformation = if (password) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        keyboardOptions = KeyboardOptions(keyboardType = if (password) KeyboardType.Password else KeyboardType.Text),
        modifier = Modifier
            .fillMaxWidth()
            .then(if (tv) Modifier.remoteLeavesField(value.isEmpty()).then(if (start) Modifier.focusOnStart() else Modifier) else Modifier),
    )
}

@Composable
private fun SaveRow(save: SaveState?, saved: Int, failed: Int, button: @Composable () -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically) {
        button()
        when (save?.status) {
            LoadStatus.Loading -> CircularProgressIndicator()
            LoadStatus.Loaded -> Text(stringResource(saved), color = Tokens.Palette.success)
            LoadStatus.Failed -> Text(save.problem?.let { problemMessage(it) } ?: stringResource(failed), color = Tokens.Palette.danger)
            else -> {}
        }
    }
}

@Composable
private fun Action(label: String, primary: Boolean, enabled: Boolean, onClick: () -> Unit) {
    when {
        LocalIsTv.current -> TvActionButton(label, onClick = onClick, primary = primary, enabled = enabled)
        primary -> Button(onClick = onClick, enabled = enabled) { Text(label) }
        else -> OutlinedButton(onClick = onClick, enabled = enabled) { Text(label) }
    }
}

private const val BIO_LIMIT = 2000
private const val MIN_PASSWORD = 8

/** [ProfileEditorScreen] over the core, with the photo picker on phones. */
@Composable
fun ProfileEditorRoute(onBack: (() -> Unit)?) {
    val send = rememberSend()
    val tv = LocalIsTv.current
    val session by rememberSurface<SessionView>(Surface.Session)
    val view by rememberSurface<ProfileEditorView>(Surface.ProfileEditor, open = true)
    val user = session?.user
    val profile = user?.let { rememberSurface<io.stepes.couchverse.core.ProfileView>(Surface.Profile(it.username)).value?.profile }
    var slot by rememberSaveable { mutableStateOf(ImageSlot.Avatar) }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri != null) send(Event.ImageChosen(ImageChoice(slot, uri.toString())))
    }
    ProfileEditorScreen(
        ProfileEditorState(user, profile?.avatar?.url, profile?.banner?.url, view),
        ProfileEditorActions(
            onSave = { send(Event.ProfileEditSubmitted(it)) },
            onPassword = { send(Event.PasswordChangeSubmitted(it)) },
            onPick = if (tv) {
                null
            } else {
                { which ->
                    slot = which
                    picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly))
                }
            },
            onRemove = { send(Event.ImageRemoved(ImageSlotRef(it))) },
            onBack = onBack,
        ),
    )
}
