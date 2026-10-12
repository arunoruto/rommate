---
title: rommate
layout: hextra-home
---

<div class="rm-hero" style="background-image: linear-gradient(90deg, rgba(22,22,30,.92) 0%, rgba(22,22,30,.6) 45%, rgba(22,22,30,0) 75%), url(images/hero.png);">

{{< hextra/hero-badge link="https://github.com/arunoruto/rommate" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>Free and open source · MIT</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Your <span class="rm-gradient">RomM</span> library,&nbsp;<br class="hx:sm:block hx:hidden" />on your handheld
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-10">
{{< hextra/hero-subtitle >}}
  Pick games with your controller and rommate downloads them&nbsp;<br class="hx:sm:block hx:hidden" />straight into the folders ES-DE, EmuDeck and RetroDECK expect.
{{< /hextra/hero-subtitle >}}
</div>

<div class="rm-buttons">
{{< hextra/hero-button text="Get started" link="docs/getting-started" >}}
{{< hextra/hero-button text="Download" link="https://github.com/arunoruto/rommate/releases/latest" style="background: transparent; color: inherit; box-shadow: inset 0 0 0 1px rgba(123,97,255,.55);" >}}
</div>

<div class="hx:mb-12 hx:max-w-full">
<code class="rm-install">curl -fsSL https://arunoruto.github.io/rommate/install.sh | sh</code>
</div>

</div>

<div class="hx:mb-16">
<img src="images/screenshots/games.png" alt="rommate TUI showing Nintendo 3DS games with one download in progress" class="rm-screenshot" width="1979" height="1022">
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="The right folders, automatically"
    icon="folder-open"
    subtitle="RomM and ES-DE name platforms differently: `3ds` vs `n3ds`, `ngc` vs `gc`. rommate maps every one of them and reads your ROM folder from ES-DE's own settings."
    style="background: radial-gradient(ellipse at 50% 80%,rgba(123,97,255,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Made for a controller"
    icon="sparkles"
    subtitle="D-pad, A/B/X/Y, L1/R1 and Start are all you need. Works in Steam's Game Mode through Steam Input and shows button hints for the device you use."
    style="background: radial-gradient(ellipse at 50% 80%,rgba(255,122,184,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Downloads that finish"
    icon="download"
    subtitle="A queue with progress, speed and time left. Interrupted downloads resume where they stopped, files are size-checked, and games you already have are picked up."
    style="background: radial-gradient(ellipse at 50% 80%,rgba(123,97,255,0.12),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Nothing in the background"
    icon="lightning-bolt"
    subtitle="One small static binary that only runs while you use it. No daemon, no runtime, no dependencies."
  >}}
  {{< hextra/feature-card
    title="Scriptable"
    icon="terminal"
    subtitle="Every command has a `--json` mode with progress events as JSON lines, ready for scripts and a future Decky plugin."
  >}}
  {{< hextra/feature-card
    title="Runs where you play"
    icon="device-mobile"
    subtitle="Linux on ARM handhelds like Armada and on x86 like the Steam Deck, plus macOS and Windows."
  >}}
{{< /hextra/feature-grid >}}

<section class="rm-closer">
  <h2 class="rm-section-title">A closer look</h2>
  <div class="rm-shots">
    <figure>
      <a href="docs/tui/"><img src="images/screenshots/library.png" alt="The Library tab: every platform with how many games are on the device" class="rm-screenshot" width="1979" height="1022"></a>
      <figcaption><strong>Library</strong> Every platform on your server, with how many games are already on this device.</figcaption>
    </figure>
    <figure>
      <a href="docs/tui/"><img src="images/screenshots/downloads.png" alt="The Downloads tab with running, queued, failed and finished downloads" class="rm-screenshot" width="1979" height="1022"></a>
      <figcaption><strong>Downloads</strong> Running, queued, failed and finished downloads. Cancel, retry or move a game to the front.</figcaption>
    </figure>
  </div>
</section>
