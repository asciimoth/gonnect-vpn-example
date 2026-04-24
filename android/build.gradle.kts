plugins {
	id("base")
}

val repoRoot = rootDir.parentFile
val mobilelibAar = repoRoot.resolve("build/android/mobilelib.aar")

tasks.register<Exec>("generateGoMobileAar") {
	group = "build"
	description = "Build gomobile Android bindings for mobilelib."
	workingDir = repoRoot
	commandLine("bash", "./scripts/build-android-mobilelib-aar.sh")
	inputs.dir(repoRoot.resolve("mobilelib"))
	inputs.dir(repoRoot.resolve("clientcore"))
	inputs.dir(repoRoot.resolve("transport"))
	inputs.dir(repoRoot.resolve("cfg"))
	inputs.file(repoRoot.resolve("go.mod"))
	inputs.file(repoRoot.resolve("go.sum"))
	outputs.file(mobilelibAar)
}
