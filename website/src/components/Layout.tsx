import { Link } from '@tanstack/solid-router'
import { createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { Icon } from './Icon'
import { ThemeSwitch } from '../theme'
import { audible, listen, tap } from '../music'
import { site } from '../site'

export function Layout(props: { children: JSX.Element }) {
  // The clip starts with the page, and gives way to anything else that plays
  // on the site.
  onMount(() => listen())
  return (
    <>
      <Header />
      <main>{props.children}</main>
      <Footer />
    </>
  )
}

function Header() {
  // Hairline and frosted backing only once the page has scrolled under it.
  const [scrolled, setScrolled] = createSignal(false)
  onMount(() => {
    const on = () => setScrolled(window.scrollY > 8)
    on()
    window.addEventListener('scroll', on, { passive: true })
    onCleanup(() => window.removeEventListener('scroll', on))
  })
  return (
    <header class="nav" classList={{ scrolled: scrolled() }}>
      <div class="nav-inner">
        <Link to="/" class="brand" aria-label={site.name}>
          <img src="/icon.png" width={28} height={28} alt="" />
          <span>{site.name}</span>
        </Link>
        <div class="nav-end">
          <ThemeSwitch class="nav-theme" />
          <button
            type="button"
            class="nav-music"
            classList={{ off: !audible() }}
            aria-pressed={audible()}
            aria-label={audible() ? '关掉背景音乐' : '放背景音乐'}
            title={audible() ? '关掉背景音乐' : '放背景音乐'}
            onClick={tap}
          >
            <Icon name={audible() ? 'volume' : 'mute'} size={17} />
          </button>
        </div>
      </div>
    </header>
  )
}

function Footer() {
  return (
    <footer class="footer">
      <div class="footer-inner">
        <span>{site.name} · 第三方客户端 · 与网易云音乐官方无关</span>
      </div>
    </footer>
  )
}
