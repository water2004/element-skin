/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent
  export default component
}

declare const __APP_VERSION__: string

// matter-js ships no type declarations and the easter egg only touches a small
// runtime API surface.
declare module 'matter-js' {
  interface Gravity {
    x: number
    y: number
    scale?: number
  }

  export interface Vector {
    x: number
    y: number
  }

  export interface BodyType {
    id: number
    position: Vector
    angle: number
  }

  export interface CompositeType {
    bodies: BodyType[]
  }

  export interface EngineType {
    world: CompositeType
    gravity: Gravity
  }

  export interface BodyOptions {
    [key: string]: unknown
  }

  const Matter: {
    Engine: {
      create(options?: { gravity?: Gravity }): EngineType
      clear(engine: EngineType): void
      update(engine: EngineType, delta?: number): void
    }
    Bodies: {
      rectangle(
        x: number,
        y: number,
        width: number,
        height: number,
        options?: BodyOptions,
      ): BodyType
      circle(x: number, y: number, radius: number, options?: BodyOptions): BodyType
      polygon(
        x: number,
        y: number,
        sides: number,
        radius: number,
        options?: BodyOptions,
      ): BodyType
    }
    Body: {
      rotate(body: BodyType, angle: number, point?: Vector): void
      setAngularVelocity(body: BodyType, velocity: number): void
      setPosition(body: BodyType, position: Vector): void
      translate(body: BodyType, translation: Vector): void
      setVelocity(body: BodyType, velocity: Vector): void
    }
    Composite: {
      create(): CompositeType
      add(
        composite: CompositeType,
        object: BodyType | CompositeType | Array<BodyType | CompositeType>,
      ): void
      remove(composite: CompositeType, object: BodyType | CompositeType): void
      translate(composite: CompositeType, translation: Vector): void
      allBodies(composite: CompositeType): BodyType[]
    }
    Vector: {
      create(x: number, y: number): Vector
      magnitude(vector: Vector): number
      normalise(vector: Vector): Vector
    }
  }
  export default Matter
}

interface Window {
  elementSkinEasterEggs?: {
    list: () => Array<{ id: string; name: string; description: string }>
    start: (id: string) => Promise<boolean>
    stop: () => void
    refreshAt: (date: string | Date) => Promise<void>
    setDisabled: (disabled: boolean) => void
  }
}
