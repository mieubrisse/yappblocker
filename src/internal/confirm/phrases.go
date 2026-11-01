package confirm

// UninstallPhrase has to be typed before the launchd agent will be removed. It is
// long on purpose: the cost of typing it is the whole mechanism.
const UninstallPhrase = "I understand that I am uninstalling yappblocker, and in doing so I am opening myself up to sacrificing my long-term goals in favor of short-term dopamine. I accept that now I WILL stay up late, tired, one day. It may not be today or this week. But it will happen, and yappblocker is there to protect me so that I'm aligned with my long-term goals. I understand that even if I'm tired or frustrated right now, I still have responsibility for myself and it's not an excuse to throw all the responsibility to Future Me. Knowing all this, I want to uninstall yappblocker."

// PausePhrase has to be typed before blocking is paused. It is deliberately shorter
// than UninstallPhrase, because a bounded pause is the sanctioned escape hatch and
// uninstalling is not.
const PausePhrase = "I want to pause yappblocker. I understand that by doing this, I am putting myself at risk of self-harming behaviours and bad decisions. I am doing this pause for a legitimate reason."
