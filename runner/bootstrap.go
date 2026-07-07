package runner

// PrSetNoNewPrivs is the prctl option constant for PR_SET_NO_NEW_PRIVS.
// Exported so bootstrap child processes (e.g. the compiler) can set it via
// prctl before execve.
const PrSetNoNewPrivs = 38
