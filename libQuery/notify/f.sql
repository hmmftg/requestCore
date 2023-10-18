select * from mytable;
DO $$
DECLARE s record;
BEGIN
    FOR s IN SELECT name FROM mytable
           LOOP
        RAISE NOTICE 'Tenant Name: %', s;
       END LOOP;
   END;
$$
;;
select name from mytable;