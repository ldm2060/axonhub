import { useEffect, useState } from 'react';
import { MODEL_ICON_LOADERS, type ModelIconComponent } from '../data/model-icon-loaders.generated';

const iconComponents = new Map<string, ModelIconComponent>();
const iconPromises = new Map<string, Promise<ModelIconComponent | null>>();

function loadModelIcon(name: string) {
  const cachedComponent = iconComponents.get(name);
  if (cachedComponent) {
    return Promise.resolve(cachedComponent);
  }

  const cachedPromise = iconPromises.get(name);
  if (cachedPromise) {
    return cachedPromise;
  }

  const loader = MODEL_ICON_LOADERS[name];
  if (!loader) {
    return Promise.resolve(null);
  }

  const promise = loader()
    .then(({ default: component }) => {
      iconComponents.set(name, component);
      return component;
    })
    .catch(() => null);
  iconPromises.set(name, promise);
  return promise;
}

interface ModelIconProps {
  name?: string | null;
  className?: string;
}

export function ModelIcon({ name, className }: ModelIconProps) {
  const [Icon, setIcon] = useState<ModelIconComponent | null>(() => (name ? iconComponents.get(name) || null : null));

  useEffect(() => {
    let active = true;

    if (!name) {
      setIcon(null);
      return () => {
        active = false;
      };
    }

    const cachedComponent = iconComponents.get(name);
    if (cachedComponent) {
      setIcon(() => cachedComponent);
      return () => {
        active = false;
      };
    }

    setIcon(null);
    void loadModelIcon(name).then((component) => {
      if (active) {
        setIcon(() => component);
      }
    });

    return () => {
      active = false;
    };
  }, [name]);

  if (!Icon) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  return <Icon className={className} />;
}
