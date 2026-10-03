import com.android.build.api.dsl.ApplicationExtension
import com.android.build.api.dsl.LibraryExtension
import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.artifacts.VersionCatalogsExtension
import org.gradle.kotlin.dsl.configure
import org.gradle.kotlin.dsl.dependencies
import org.gradle.kotlin.dsl.getByType

/** Compose on an Android library or application, with the BOM on every classpath. */
class AndroidComposeConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) {
        with(target) {
            pluginManager.apply("org.jetbrains.kotlin.plugin.compose")
            pluginManager.withPlugin("com.android.library") {
                extensions.configure<LibraryExtension> { buildFeatures.compose = true }
            }
            pluginManager.withPlugin("com.android.application") {
                extensions.configure<ApplicationExtension> { buildFeatures.compose = true }
            }
            val libs = extensions.getByType<VersionCatalogsExtension>().named("libs")
            fun lib(alias: String) = libs.findLibrary(alias).get()
            dependencies {
                val bom = platform(lib("androidx-compose-bom"))
                add("implementation", bom)
                add("testImplementation", bom)
                add("implementation", lib("androidx-compose-ui-tooling-preview"))
                add("debugImplementation", lib("androidx-compose-ui-tooling"))
            }
        }
    }
}
