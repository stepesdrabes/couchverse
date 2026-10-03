# The navigation routes and the core's message types are kotlinx-serialization classes; the
# library ships its own rules for generated serializers, the routes only need their names kept.
-keepnames class io.stepes.couchverse.navigation.** { *; }
