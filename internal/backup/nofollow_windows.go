package backup

// Windows has no O_NOFOLLOW. Restore is supported on Linux hosts; this keeps
// the package building for development on Windows.
const openNoFollow = 0
