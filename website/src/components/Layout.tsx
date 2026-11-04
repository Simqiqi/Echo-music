import { Link } from '@tanstack/solid-router'
import { createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import { ThemeSwitch } from '../theme'
import { site } from '../site'

export function Layout(props: { children: JSX.Element }) {
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
        </div>
      </div>
    </header>
  )
}

function Footer() {
  return (
    <footer class="footer">
      <div class="footer-inner">
        <span>
          {site.name} · 第三方客户端 · 与网易云音乐官方无关
        </span>
      </div>
    </footer>
  )
}
