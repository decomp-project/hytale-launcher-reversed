package pkg

// GetGameManifest returns the game version manifest getter.
func GetGameManifest() interface{} {
	return gameManifest
}

// GetJavaManifest returns the Java version manifest getter.
func GetJavaManifest() interface{} {
	return javaManifest
}

// GetLauncherManifest returns the launcher version manifest getter.
func GetLauncherManifest() interface{} {
	return launcherManifest
}
