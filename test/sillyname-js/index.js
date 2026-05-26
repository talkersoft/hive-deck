import sillyname from 'sillyname';

for (let i = 1; i <= 50; i++) {
  console.log(`${String(i).padStart(2)}. ${sillyname().toLowerCase().replace(' ', '-')}`);
}
